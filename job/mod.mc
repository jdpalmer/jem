// job — background compile/grep via os.Cmd + Poll (non-blocking main loop).
import "buffer";
import "display";
import "window";
import "markring";
import "file";
import "fs";
import "os";
import "path";
import "str";
import "syntax";

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>

enum {
	JobOutCap = 1024 * 1024,
	JobErrCap = 16 * 1024,
	JobPathMax = 4096
};

enum JobKind {
	JobNone = 0,
	JobGrep = 1,
	JobCompile = 2,
	JobTags = 3
};

/* Contiguous blob: JobLocHead then NUL-terminated path (freed via free()). */
struct JobLocHead {
	int line;
	int column;
};

static int g_job_kind = JobNone;
static Cmd g_job_cmd = {
	0 };
static File g_job_out = {
	0 };
static bool g_job_out_open;
static File g_job_err = {
	0 };
static bool g_job_err_open;
static char *g_job_buf = NULL;
static size_t g_job_len = 0;
static char *g_job_err_buf = NULL;
static size_t g_job_err_len = 0;
static char g_job_root[JobPathMax] = {
	0 };
static char g_job_arg0[512] = {
	0 };
static char g_last_compile[256] = {
	0 };
static char g_job_tags_target[JobPathMax] = {
	0 };
static char g_job_tags_temp[JobPathMax] = {
	0 };
static bool g_job_tags_changed;

static bool job_accum_append(char *src, size_t n) {
	size_t need = 0;
	char *p = NULL;
	if (n == 0 || src == NULL) {
		return true;
	}
	if (g_job_len + n > JobOutCap) {
		n = JobOutCap - g_job_len;
		if (n == 0) {
			return true;
		}
	}
	need = g_job_len + n + 1;
	if (need > JobOutCap + 1) {
		need = JobOutCap + 1;
	}
	p = realloc(g_job_buf, need);
	if (p == NULL) {
		return false;
	}
	memcpy(p + g_job_len, src, n);
	g_job_len = g_job_len + n;
	p[g_job_len] = 0;
	g_job_buf = p;
	return true;
}

static bool job_err_append(char *src, size_t n) {
	size_t need = 0;
	char *p = NULL;
	if (n == 0 || src == NULL) {
		return true;
	}
	if (g_job_err_len + n > (size_t)JobErrCap) {
		n = (size_t)JobErrCap - g_job_err_len;
		if (n == 0) {
			return true;
		}
	}
	need = g_job_err_len + n + 1;
	p = realloc(g_job_err_buf, need);
	if (p == NULL) {
		return false;
	}
	memcpy(p + g_job_err_len, src, n);
	g_job_err_len += n;
	p[g_job_err_len] = 0;
	g_job_err_buf = p;
	return true;
}

static void job_clear() {
	if (g_job_out_open) {
		g_job_out.close();
		g_job_out_open = false;
	}
	if (g_job_err_open) {
		g_job_err.close();
		g_job_err_open = false;
	}
	g_job_cmd.close();
	g_job_kind = JobNone;
}

static bool job_project_root(char *dst, size_t cap) {
	Buffer *cur = buffer_current();
	char dir[JobPathMax] = {
		0 };
	char[..] logical = str_empty();

	if (cur != NULL && len(cur.file_name) > 0) {
		logical = cur.file_name;
		{
			char[..] d = path_dir(logical);
			if (len(d) > 0 && ptr(d) != NULL) {
				int n = snprintf(dst, cap, "%s", d);
				if (n >= 0 && (size_t)n < cap) {
					return true;
				}
			}
		}
	}
	{
		auto (cw, ok) = os_cwd(ranged(dir, 0, sizeof(dir)));
		int n = 0;
		if (!ok || len(cw) == 0) {
			return false;
		}
		n = snprintf(dst, cap, "%s", cw);
		if (n < 0 || (size_t)n >= cap) {
			return false;
		}
	}
	return true;
}

static void *job_loc_alloc(char *path, int line, int column) {
	size_t plen = 0;
	JobLocHead *h = NULL;
	if (path == NULL) {
		return NULL;
	}
	plen = strlen(path);
	h = malloc(sizeof(JobLocHead) + plen + 1);
	if (h == NULL) {
		return NULL;
	}
	h.line = line;
	h.column = column;
	memcpy((char *)(h + 1), path, plen + 1);
	return h;
}

static char *job_loc_path(JobLocHead *h) {
	if (h == NULL) {
		return NULL;
	}
	return (char *)(h + 1);
}

static bool job_path_is_abs(char *p) {
	if (p == NULL || p[0] == 0) {
		return false;
	}
	if (p[0] == '/') {
		return true;
	}
	return false;
}

static bool job_abs_path(char *dst, size_t cap, char *rel) {
	char joined[JobPathMax] = {
		0 };
	if (rel == NULL || dst == NULL || cap < 2) {
		return false;
	}
	if (job_path_is_abs(rel)) {
		if (strlen(rel) >= cap) {
			return false;
		}
		snprintf(dst, cap, "%s", rel);
		return true;
	}
	{
		auto (jv, jok) = path_join(ranged(joined, 0, sizeof(joined)),
			str_from_cstr(g_job_root), str_from_cstr(rel));
		int n = 0;
		if (!jok || len(jv) == 0) {
			return false;
		}
		n = snprintf(dst, cap, "%s", jv);
		if (n < 0 || (size_t)n >= cap) {
			return false;
		}
	}
	return true;
}

/* Parse tool line: path:line:text or path:line:col:text. */
static bool job_parse_colon_hit(char *text, char *path_out, size_t path_cap,
	int *line_out, int *col_out, char **msg_out) {
	size_t i = 0;
	size_t path_end = 0;
	int line = 0;
	int col = 1;
	size_t j = 0;
	bool has_col = false;
	int parsed_col = 0;

	if (text == NULL || text[0] == 0 || path_out == NULL || line_out == NULL || col_out == NULL) {
		return false;
	}
	while (text[i] != 0 && text[i] != ':') {
		i++;
	}
	if (i == 0 || text[i] != ':') {
		return false;
	}
	path_end = i;
	if (path_end >= path_cap) {
		return false;
	}
	memcpy(path_out, text, path_end);
	path_out[path_end] = 0;
	i++;
	j = i;
	while (text[j] >= '0' && text[j] <= '9') {
		line = line * 10 + (text[j] - '0');
		j++;
	}
	if (line == 0 || text[j] != ':') {
		return false;
	}
	j++;
	parsed_col = 0;
	has_col = false;
	i = j;
	while (text[i] >= '0' && text[i] <= '9') {
		parsed_col = parsed_col * 10 + (text[i] - '0');
		has_col = true;
		i++;
	}
	if (has_col && parsed_col > 0 && text[i] == ':') {
		col = parsed_col;
		i++;
	} else {
		col = 1;
		i = j;
	}
	*line_out = line;
	*col_out = col;
	if (msg_out != NULL) {
		*msg_out = text + i;
	}
	return true;
}

static Buffer *job_ensure_named(const char[..] name) {
	Buffer *buf = buffer_find_by_name(name);
	if (buf != NULL) {
		return buf;
	}
	buf = buffer_new();
	if (buf == NULL) {
		return NULL;
	}
	buf.set_name(name);
	if (!buffer_list_add(buf)) {
		buf.free();
		return NULL;
	}
	return buf;
}

static bool job_set_line1(Buffer *buf, char *text) {
	Line *line = {0};
	Location begin = {0};
	Location end = {0};
	if (buf == NULL || text == NULL) {
		return false;
	}
	line = buf.line(1);
	if (line == NULL) {
		return false;
	}
	begin.line = 1;
	begin.offset = 0;
	end.line = 1;
	end.offset = line.byte_len();
	return buf.replace_raw(begin, end, text, strlen(text), NULL);
}

static void job_present_buffer(Buffer *buf) {
	Window *w = {0};
	Location loc = {0};
	if (buf == NULL) {
		return;
	}
	{
		Window *cur = window_current();
		if (cur != NULL) {
			markring_push(cur.cursor);
		}
	}
	buffer_set_current(buf);
	window_switch_buffer(buf);
	w = window_current();
	if (w == NULL) {
		return;
	}
	loc.line = 1;
	loc.offset = 0;
	w.top_line = 1;
	w.set_cursor(loc);
	w.mark.line = 0;
	w.mark.offset = 0;
	w.h_scroll = 0;
	w.should_redraw = true;
	w.should_update_mode_line = true;
}

static int job_fill_grep(Buffer *buf, char *raw, size_t n, char *pattern, bool truncated) {
	size_t off = 0;
	char path[JobPathMax] = {
		0 };
	char abs[JobPathMax] = {
		0 };
	char current_file[JobPathMax] = {
		0 };
	char linebuf[1024] = {
		0 };
	char summary[512] = {
		0 };
	char empty_msg[1] = {0};
	int match_count = 0;
	int file_count = 0;
	int last_line = 0;
	bool have_last = false;
	char *msg = NULL;
	int hit_line = 0;
	int hit_col = 0;

	if (buf == NULL) {
		return -1;
	}
	buf.clear();
	buf.set_file_name(str_empty());
	buf.lang_mode = LModeMarkdown;
	buf.is_changed = false;

	while (off < n) {
		size_t start = off;
		size_t len_line = 0;
		Line *ln = NULL;
		char *raw_line = NULL;

		while (off < n && raw[off] != '\n') {
			off++;
		}
		len_line = off - start;
		if (off < n && raw[off] == '\n') {
			off++;
		}
		if (len_line == 0) {
			continue;
		}
		if (len_line >= sizeof(linebuf)) {
			len_line = sizeof(linebuf) - 1;
		}
		memcpy(linebuf, raw + start, len_line);
		linebuf[len_line] = 0;
		if (len_line > 0 && linebuf[len_line - 1] == '\r') {
			linebuf[len_line - 1] = 0;
		}
		raw_line = linebuf;
		if (!job_parse_colon_hit(raw_line, path, sizeof(path), &hit_line, &hit_col, &msg)) {
			continue;
		}
		if (msg == NULL) {
			msg = empty_msg;
		}
		if (strcmp(path, current_file) != 0) {
			snprintf(current_file, sizeof(current_file), "%s", path);
			have_last = false;
			if (match_count > 0) {
				if (buf.append_line("") == NULL) {
					return -1;
				}
			}
			{
				char header[JobPathMax + 8] = {
					0 };
				snprintf(header, sizeof(header), "## %s", path);
				if (buf.append_line(str_from_cstr(header)) == NULL) {
					return -1;
				}
			}
			file_count++;
		}
		if (have_last && last_line == hit_line) {
			continue;
		}
		{
			char out[768] = {
				0 };
			size_t mlen = strlen(msg);
			if (mlen > 512) {
				mlen = 512;
			}
			snprintf(out, sizeof(out), "L%d: ", hit_line);
			{
				size_t prefix = strlen(out);
				if (prefix + mlen >= sizeof(out)) {
					mlen = sizeof(out) - prefix - 1;
				}
				memcpy(out + prefix, msg, mlen);
				out[prefix + mlen] = 0;
			}
			ln = buf.append_line(str_from_cstr(out));
			if (ln == NULL) {
				return -1;
			}
		}
		if (!job_abs_path(abs, sizeof(abs), path)) {
			snprintf(abs, sizeof(abs), "%s", path);
		}
		ln.metadata = job_loc_alloc(abs, hit_line, hit_col);
		last_line = hit_line;
		have_last = true;
		match_count++;
	}

	snprintf(summary, sizeof(summary), "# %d matches across %d files for `%s`",
		match_count, file_count, pattern != NULL ? pattern : "");
	if (!job_set_line1(buf, summary)) {
		return -1;
	}
	if (match_count == 0) {
		char msg_no[300] = {
			0 };
		snprintf(msg_no, sizeof(msg_no), "[no matches for: %s]",
			pattern != NULL ? pattern : "");
		if (buf.append_line(str_from_cstr(msg_no)) == NULL) {
			return -1;
		}
	}
	if (truncated) {
		if (buf.append_line("[results truncated]") == NULL) {
			return -1;
		}
	}
	buf.is_changed = false;
	buf.cursor.line = 1;
	buf.cursor.offset = 0;
	buf.mark.line = 0;
	buf.mark.offset = 0;
	return match_count;
}

static int job_fill_compile(Buffer *buf, char *raw, size_t n, char *command, int exit_code,
	bool truncated) {
	size_t off = 0;
	char path[JobPathMax] = {
		0 };
	char abs[JobPathMax] = {
		0 };
	char linebuf[1024] = {
		0 };
	char summary[256] = {
		0 };
	int diag = 0;
	int errors = 0;
	int warnings = 0;
	char *msg = NULL;
	int hit_line = 0;
	int hit_col = 0;

	if (buf == NULL) {
		return -1;
	}
	buf.clear();
	buf.set_file_name(str_empty());
	buf.lang_mode = LModeMarkdown;
	buf.is_changed = false;

	{
		char cmd_line[600] = {
			0 };
		snprintf(cmd_line, sizeof(cmd_line), "$ %s", command != NULL ? command : "");
		if (buf.append_line(str_from_cstr(cmd_line)) == NULL) {
			return -1;
		}
	}
	if (buf.append_line("") == NULL) {
		return -1;
	}
	{
		if (buf.append_line("## stdout") == NULL) {
			return -1;
		}
	}

	while (off <= n) {
		size_t start = off;
		size_t len_line = 0;
		Line *ln = NULL;

		if (off == n) {
			break;
		}
		while (off < n && raw[off] != '\n') {
			off++;
		}
		len_line = off - start;
		if (off < n && raw[off] == '\n') {
			off++;
		}
		if (len_line >= sizeof(linebuf)) {
			len_line = sizeof(linebuf) - 1;
		}
		memcpy(linebuf, raw + start, len_line);
		linebuf[len_line] = 0;
		if (len_line > 0 && linebuf[len_line - 1] == '\r') {
			linebuf[len_line - 1] = 0;
			len_line--;
		}
		ln = buf.append_line(str_from_cstr(linebuf));
		if (ln == NULL) {
			return -1;
		}
		if (linebuf[0] != 0 &&
			job_parse_colon_hit(linebuf, path, sizeof(path), &hit_line, &hit_col, &msg)) {
			diag++;
			{
				char *lower = linebuf;
				if (strstr(lower, "error") != NULL || strstr(lower, "Error") != NULL) {
					errors++;
				} else if (strstr(lower, "warning") != NULL || strstr(lower, "Warning") != NULL) {
					warnings++;
				}
			}
			if (!job_abs_path(abs, sizeof(abs), path)) {
				snprintf(abs, sizeof(abs), "%s", path);
			}
			ln.metadata = job_loc_alloc(abs, hit_line, hit_col);
		}
	}

	if (truncated) {
		if (buf.append_line("") == NULL) {
			return -1;
		}
		{
			if (buf.append_line("[output truncated]") == NULL) {
				return -1;
			}
		}
	}

	snprintf(summary, sizeof(summary),
		"# compile exit=%d, diagnostics=%d, errors=%d, warnings=%d",
		exit_code, diag, errors, warnings);
	if (!job_set_line1(buf, summary)) {
		return -1;
	}
	buf.is_changed = false;
	buf.cursor.line = 1;
	buf.cursor.offset = 0;
	return diag;
}

static void job_finish(int exit_code) {
	Buffer *buf = NULL;
	bool was_ro = false;
	bool truncated = false;
	char echo[256] = {
		0 };
	int kind = g_job_kind;
	int matches = 0;
	int diags = 0;

	truncated = g_job_len >= JobOutCap;
	if (kind == (int)JobGrep) {
		buf = job_ensure_named("*grep*");
		if (buf == NULL) {
			display_set_echo("[grep failed]");
		} else {
			was_ro = buf.is_readonly;
			buf.is_readonly = false;
			matches = job_fill_grep(buf, g_job_buf, g_job_len, g_job_arg0, truncated);
			if (matches < 0) {
				buf.is_readonly = was_ro;
				display_set_echo("[grep failed]");
			} else {
				buf.is_readonly = true;
				job_present_buffer(buf);
				if (matches == 1) {
					snprintf(echo, sizeof(echo), "[1 match]");
				} else {
					snprintf(echo, sizeof(echo), "[%d matches]", matches);
				}
				display_set_echo(echo);
			}
		}
	} else if (kind == (int)JobCompile) {
		buf = job_ensure_named("*compile*");
		if (buf == NULL) {
			display_set_echo("[compile failed]");
		} else {
			was_ro = buf.is_readonly;
			buf.is_readonly = false;
			diags = job_fill_compile(buf, g_job_buf, g_job_len, g_job_arg0, exit_code, truncated);
			if (diags < 0) {
				buf.is_readonly = was_ro;
				display_set_echo("[compile failed]");
			} else {
				buf.is_readonly = true;
				job_present_buffer(buf);
				if (exit_code == 0) {
					snprintf(echo, sizeof(echo), "[compile ok: %d diagnostics]", diags);
				} else {
					snprintf(echo, sizeof(echo), "[compile exit %d: %d diagnostics]", exit_code, diags);
				}
				display_set_echo(echo);
			}
		}
	} else if (kind == (int)JobTags) {
		if (exit_code != 0) {
			if (g_job_err_buf != NULL && g_job_err_buf[0] != 0) {
				snprintf(echo, sizeof(echo), "[tags failed: %.200s]",
					g_job_err_buf);
			} else {
				snprintf(echo, sizeof(echo), "[tags failed: exit %d]",
					exit_code);
			}
			fs_remove(str_from_cstr(g_job_tags_temp));
		} else if (!fs_rename(str_from_cstr(g_job_tags_temp),
				str_from_cstr(g_job_tags_target))) {
			snprintf(echo, sizeof(echo), "[tags failed: cannot replace tags file]");
			fs_remove(str_from_cstr(g_job_tags_temp));
		} else {
			snprintf(echo, sizeof(echo), "[tags ok: %s]", g_job_tags_target);
			g_job_tags_changed = true;
		}
		g_job_tags_temp[0] = 0;
		g_job_tags_target[0] = 0;
		display_set_echo(echo);
	} else {
		snprintf(echo, sizeof(echo), "[job done exit=%d]", exit_code);
		display_set_echo(echo);
	}
	free(g_job_buf);
	free(g_job_err_buf);
	g_job_buf = NULL;
	g_job_len = 0;
	g_job_err_buf = NULL;
	g_job_err_len = 0;
	job_clear();
}

static bool job_start_argv(const char[..] program, const char[..] argv[], size_t nargs,
	char *workdir, int kind) {
	char prog_path[JobPathMax] = {
		0 };
	const char[..] resolved = program;
	Cmd *c = &g_job_cmd;
	size_t i = 0;
	const char[..] stack_args[12] = {
		0 };

	if (g_job_kind != (int)JobNone) {
		display_set_echo("[busy: background job running]");
		return false;
	}
	if (nargs > 12) {
		return false;
	}
	free(g_job_buf);
	free(g_job_err_buf);
	g_job_buf = NULL;
	g_job_len = 0;
	g_job_err_buf = NULL;
	g_job_err_len = 0;
	g_job_kind = kind;

	{
		auto (found, lok) = os_look_path(
			ranged(prog_path, 0, sizeof(prog_path)), program);
		(void)found;
		if (!lok) {
			display_set_echo("[job: program not found]");
			g_job_kind = JobNone;
			return false;
		}
		resolved = str_from_cstr(prog_path);
	}
	for (i = 0; i < nargs; i++) {
		stack_args[i] = argv[i];
	}
	c.init(resolved, stack_args, nargs);
	if (!c.dir(str_from_cstr(workdir))) {
		display_set_echo("[job: bad working directory]");
		job_clear();
		return false;
	}
	{
		auto (ef, ok) = c.pipe(StdErr);
		if (!ok) {
			display_set_echo("[job: stderr pipe failed]");
			job_clear();
			return false;
		}
		g_job_err = ef;
		g_job_err_open = true;
	}
	{
		auto (pf, ok) = c.pipe(StdOut);
		if (!ok) {
			display_set_echo("[job: pipe failed]");
			job_clear();
			return false;
		}
		g_job_out = pf;
		g_job_out_open = true;
	}
	if (!c.start()) {
		display_set_echo("[job: start failed]");
		job_clear();
		return false;
	}
	display_set_echo("[running...] (C-g to cancel)");
	return true;
}

void job_init() {
	g_last_compile[0] = 0;
	snprintf(g_last_compile, sizeof(g_last_compile), "make -k");
	job_clear();
}

bool job_running() {
	return g_job_kind != (int)JobNone;
}

bool job_cancel() {
	int kind = g_job_kind;
	if (!job_running()) {
		return false;
	}
	g_job_cmd.close();
	if (kind == (int)JobTags && g_job_tags_temp[0] != 0) {
		fs_remove(str_from_cstr(g_job_tags_temp));
		g_job_tags_temp[0] = 0;
		g_job_tags_target[0] = 0;
	}
	free(g_job_buf);
	free(g_job_err_buf);
	g_job_buf = NULL;
	g_job_len = 0;
	g_job_err_buf = NULL;
	g_job_err_len = 0;
	job_clear();
	display_set_echo("[cancelled]");
	return true;
}

bool job_take_tags_changed() {
	bool changed = g_job_tags_changed;
	g_job_tags_changed = false;
	return changed;
}

bool job_start_tags(char *ctags, char *root, char *target, char *temp,
	char *exclude_name) {
	char exclude_arg[JobPathMax + 16] = {0};
	const char[..] argv[11] = {0};
	bool ok = false;
	if (ctags == NULL || root == NULL || target == NULL || temp == NULL ||
		exclude_name == NULL) {
		return false;
	}
	if (g_job_kind != (int)JobNone) {
		display_set_echo("[busy: background job running]");
		fs_remove(str_from_cstr(temp));
		return false;
	}
	snprintf(exclude_arg, sizeof(exclude_arg), "--exclude=%s", exclude_name);
	argv[0] = "--options=NONE";
	argv[1] = "-R";
	argv[2] = "--exclude=.git";
	argv[3] = str_from_cstr(exclude_arg);
	argv[4] = "--exclude=.jem-tags-*";
	argv[5] = "--output-format=json";
	argv[6] = "--fields=+nKSzastP";
	argv[7] = "--extras=+q";
	argv[8] = "-f";
	argv[9] = str_from_cstr(temp);
	argv[10] = ".";
	snprintf(g_job_tags_target, sizeof(g_job_tags_target), "%s", target);
	snprintf(g_job_tags_temp, sizeof(g_job_tags_temp), "%s", temp);
	ok = job_start_argv(str_from_cstr(ctags), argv, 11, root, JobTags);
	if (!ok) {
		fs_remove(str_from_cstr(temp));
		g_job_tags_temp[0] = 0;
		g_job_tags_target[0] = 0;
		return false;
	}
	display_set_echo("[building tags...] (C-g to cancel)");
	return true;
}

char *job_last_compile_command() {
	return g_last_compile;
}

bool job_start_grep(char *pattern) {
	char rg[JobPathMax] = {
		0 };
	char grep[JobPathMax] = {
		0 };
	const char[..] argv_rg[6] = {
		0 };
	const char[..] argv_grep[4] = {
		0 };
	if (pattern == NULL || pattern[0] == 0) {
		display_set_echo("[empty grep pattern]");
		return false;
	}
	snprintf(g_job_arg0, sizeof(g_job_arg0), "%s", pattern);
	if (!job_project_root(g_job_root, sizeof(g_job_root))) {
		display_set_echo("[grep: no project root]");
		return false;
	}
	{
		auto (found, rok) = os_look_path(ranged(rg, 0, sizeof(rg)), "rg");
		(void)found;
		if (rok) {
			argv_rg[0] = "-n";
			argv_rg[1] = "--column";
			argv_rg[2] = "--no-heading";
			argv_rg[3] = "--color=never";
			argv_rg[4] = str_from_cstr(g_job_arg0);
			argv_rg[5] = ".";
			return job_start_argv("rg", argv_rg, 6, g_job_root, JobGrep);
		}
	}
	{
		auto (found, gok) = os_look_path(ranged(grep, 0, sizeof(grep)), "grep");
		(void)found;
		if (!gok) {
			display_set_echo("[grep: need rg or grep on PATH]");
			return false;
		}
		argv_grep[0] = "-rn";
		argv_grep[1] = str_from_cstr(g_job_arg0);
		argv_grep[2] = ".";
		return job_start_argv("grep", argv_grep, 3, g_job_root, JobGrep);
	}
}

bool job_start_compile(char *command) {
	char sh[JobPathMax] = {
		0 };
	const char[..] argv[2] = {
		0 };
	if (command == NULL || command[0] == 0) {
		display_set_echo("[empty compile command]");
		return false;
	}
	snprintf(g_last_compile, sizeof(g_last_compile), "%s", command);
	snprintf(g_job_arg0, sizeof(g_job_arg0), "%s", command);
	if (!job_project_root(g_job_root, sizeof(g_job_root))) {
		display_set_echo("[compile: no project root]");
		return false;
	}
	{
		auto (found, ok) = os_look_path(ranged(sh, 0, sizeof(sh)), "/bin/sh");
		(void)found;
		if (!ok) {
			display_set_echo("[compile: /bin/sh not found]");
			return false;
		}
		argv[0] = "-c";
		argv[1] = str_from_cstr(g_job_arg0);
		return job_start_argv(str_from_cstr(sh), argv, 2, g_job_root,
			JobCompile);
	}
}

/* job_cursor_loc: jump target on the current *grep* or *compile* line. */
bool job_cursor_loc(char *path_out, size_t path_cap, int *line_out, int *col_out) {
	Buffer *buf = buffer_current();
	Window *w = window_current();
	Line *line = NULL;
	JobLocHead *h = NULL;
	char *p = NULL;

	if (buf == NULL || w == NULL || path_out == NULL || path_cap < 2 || line_out == NULL || col_out == NULL) {
		return false;
	}
	if (len(buf.name) == 0) {
		return false;
	}
	if (!str_eq(buf.name, "*grep*") && !str_eq(buf.name, "*compile*")) {
		return false;
	}
	line = buf.line(w.cursor.line);
	if (line == NULL || line.metadata == NULL) {
		return false;
	}
	h = (JobLocHead *)line.metadata;
	p = job_loc_path(h);
	if (p == NULL || p[0] == 0) {
		return false;
	}
	if (strlen(p) >= path_cap) {
		return false;
	}
	snprintf(path_out, path_cap, "%s", p);
	*line_out = h.line;
	*col_out = h.column;
	return true;
}

bool job_visit_at_point() {
	char path[JobPathMax] = {
		0 };
	int line = 0;
	int col = 0;
	Window *w = window_current();
	if (!job_cursor_loc(path, sizeof(path), &line, &col)) {
		return false;
	}
	if (w != NULL) {
		markring_push(w.cursor);
	}
	return file_visit_location(path, line, col);
}

static bool job_read_ready(File *file) {
	char chunk[4096] = {0};
	if (file == &g_job_out) {
		auto (n, ok) = g_job_out.read(ranged(chunk, sizeof(chunk)));
		return ok && n > 0 && job_accum_append(chunk, n);
	}
	if (file == &g_job_err) {
		auto (n, ok) = g_job_err.read(ranged(chunk, sizeof(chunk)));
		return ok && n > 0 && job_err_append(chunk, n);
	}
	return false;
}

void job_poll() {
	Poll pol = {
		0 };
	int exit_code = 0;
	bool done = false;

	if (!job_running()) {
		return;
	}
	pol.init();
	if (g_job_out_open && file_fd(&g_job_out) >= 0) {
		pol.add_file(&g_job_out, PollIn);
	}
	if (g_job_err_open && file_fd(&g_job_err) >= 0) {
		pol.add_file(&g_job_err, PollIn);
	}
	pol.add_cmd(&g_job_cmd);
	{
		auto (ready, ok) = pol.wait(0);
		if (!ok) {
			pol.close();
			return;
		}
		if (ready > 0) {
			int i = 0;
			for (i = 0; i < pol.ready_count(); i++) {
				auto (file, events) = pol.ready_file(i);
				Cmd *jc = pol.ready_cmd(i);
				(void)events;
				(void)job_read_ready(file);
				if (jc != NULL) {
					done = true;
				}
			}
		}
	}
	pol.close();
	if (!done) {
		return;
	}
	{
		auto (code, ok) = g_job_cmd.wait();
		if (ok) {
			exit_code = code;
		}
	}
	for (;;) {
		Poll drain = {0};
		bool read_any = false;
		drain.init();
		if (g_job_out_open && file_fd(&g_job_out) >= 0) {
			drain.add_file(&g_job_out, PollIn);
		}
		if (g_job_err_open && file_fd(&g_job_err) >= 0) {
			drain.add_file(&g_job_err, PollIn);
		}
		auto (ready, ok) = drain.wait(0);
		if (ok && ready > 0) {
			int i = 0;
			for (i = 0; i < drain.ready_count(); i++) {
				auto (file, events) = drain.ready_file(i);
				(void)events;
				if (job_read_ready(file)) {
					read_any = true;
				}
			}
		}
		drain.close();
		if (!read_any) {
			break;
		}
	}
	if (g_job_kind == (int)JobCompile && g_job_err_len > 0) {
		if (g_job_len > 0) {
			char newline[1] = {'\n'};
			(void)job_accum_append(newline, 1);
		}
		(void)job_accum_append(g_job_err_buf, g_job_err_len);
	}
	job_finish(exit_code);
}
