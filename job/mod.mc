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
	JobPathMax = 4096
};

enum JobKind {
	JobNone = 0,
	JobGrep = 1,
	JobCompile = 2
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
static char *g_job_buf = NULL;
static size_t g_job_len = 0;
static char g_job_root[JobPathMax] = {
	0 };
static char g_job_arg0[512] = {
	0 };
static char g_last_compile[256] = {
	0 };

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

static void job_clear(void) {
	if (g_job_out_open) {
		g_job_out.close();
		g_job_out_open = false;
	}
	g_job_cmd.close();
	g_job_kind = JobNone;
}

static bool job_project_root(char *dst, size_t cap) {
	Buffer *cur = buffer_current();
	char dir[JobPathMax] = {
		0 };
	char[..] logical = str_empty();

	if (cur != NULL && cur.file_name != NULL && cur.file_name[0] != 0) {
		logical = str_from_cstr(cur.file_name);
		{
			char[..] d = path_dir(logical);
			if (len(d) > 0 && ptr(d) != NULL) {
				if (len(d) + 1 < cap) {
					memcpy(dst, ptr(d), len(d));
					dst[len(d)] = 0;
					return true;
				}
			}
		}
	}
	{
		auto (cw, ok) = os_cwd(ranged(dir, 0, sizeof(dir)));
		if (!ok || len(cw) == 0 || len(cw) >= cap) {
			return false;
		}
		memcpy(dst, ptr(cw), len(cw));
		dst[len(cw)] = 0;
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
		if (!jok || len(jv) == 0 || len(jv) >= cap) {
			return false;
		}
		memcpy(dst, ptr(jv), len(jv));
		dst[len(jv)] = 0;
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
		line = line * 10 + (int)(text[j] - '0');
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
		parsed_col = parsed_col * 10 + (int)(text[i] - '0');
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

static Buffer *job_ensure_named(char *name) {
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
	free(buf.file_name);
	buf.file_name = NULL;
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
			msg = (char *)"";
		}
		if (strcmp(path, current_file) != 0) {
			snprintf(current_file, sizeof(current_file), "%s", path);
			have_last = false;
			if (match_count > 0) {
				if (buf.append_line(NULL, 0) == NULL) {
					return -1;
				}
			}
			{
				char header[JobPathMax + 8] = {
					0 };
				snprintf(header, sizeof(header), "## %s", path);
				if (buf.append_line(header, strlen(header)) == NULL) {
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
			ln = buf.append_line(out, strlen(out));
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
		if (buf.append_line(msg_no, strlen(msg_no)) == NULL) {
			return -1;
		}
	}
	if (truncated) {
		char *trunc = (char *)"[results truncated]";
		if (buf.append_line(trunc, strlen(trunc)) == NULL) {
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
	free(buf.file_name);
	buf.file_name = NULL;
	buf.lang_mode = LModeMarkdown;
	buf.is_changed = false;

	{
		char cmd_line[600] = {
			0 };
		snprintf(cmd_line, sizeof(cmd_line), "$ %s", command != NULL ? command : "");
		if (buf.append_line(cmd_line, strlen(cmd_line)) == NULL) {
			return -1;
		}
	}
	if (buf.append_line(NULL, 0) == NULL) {
		return -1;
	}
	{
		char *heading = (char *)"## stdout";
		if (buf.append_line(heading, strlen(heading)) == NULL) {
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
		ln = buf.append_line(linebuf, strlen(linebuf));
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
		if (buf.append_line(NULL, 0) == NULL) {
			return -1;
		}
		{
			char *t = (char *)"[output truncated]";
			if (buf.append_line(t, strlen(t)) == NULL) {
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
	if (kind == JobGrep) {
		buf = job_ensure_named((char *)"*grep*");
		if (buf == NULL) {
			display_set_echo(str_from_cstr((char *)"[grep failed]"));
		} else {
			was_ro = buf.is_readonly;
			buf.is_readonly = false;
			matches = job_fill_grep(buf, g_job_buf, g_job_len, g_job_arg0, truncated);
			if (matches < 0) {
				buf.is_readonly = was_ro;
				display_set_echo(str_from_cstr((char *)"[grep failed]"));
			} else {
				buf.is_readonly = true;
				job_present_buffer(buf);
				if (matches == 1) {
					snprintf(echo, sizeof(echo), "[1 match]");
				} else {
					snprintf(echo, sizeof(echo), "[%d matches]", matches);
				}
				display_set_echo(str_from_cstr(echo));
			}
		}
	} else if (kind == JobCompile) {
		buf = job_ensure_named((char *)"*compile*");
		if (buf == NULL) {
			display_set_echo(str_from_cstr((char *)"[compile failed]"));
		} else {
			was_ro = buf.is_readonly;
			buf.is_readonly = false;
			diags = job_fill_compile(buf, g_job_buf, g_job_len, g_job_arg0, exit_code, truncated);
			if (diags < 0) {
				buf.is_readonly = was_ro;
				display_set_echo(str_from_cstr((char *)"[compile failed]"));
			} else {
				buf.is_readonly = true;
				job_present_buffer(buf);
				if (exit_code == 0) {
					snprintf(echo, sizeof(echo), "[compile ok: %d diagnostics]", diags);
				} else {
					snprintf(echo, sizeof(echo), "[compile exit %d: %d diagnostics]", exit_code, diags);
				}
				display_set_echo(str_from_cstr(echo));
			}
		}
	} else {
		snprintf(echo, sizeof(echo), "[job done exit=%d]", exit_code);
		display_set_echo(str_from_cstr(echo));
	}
	free(g_job_buf);
	g_job_buf = NULL;
	g_job_len = 0;
	job_clear();
}

static bool job_start_argv(char *prog, char *argv[], size_t nargs, char *workdir, int kind) {
	char prog_path[JobPathMax] = {
		0 };
	Cmd *c = &g_job_cmd;
	size_t i = 0;
	char[..] stack_args[8] = {
		0 };

	if (g_job_kind != JobNone) {
		display_set_echo(str_from_cstr((char *)"[busy: background job running]"));
		return false;
	}
	if (nargs > 8) {
		return false;
	}
	free(g_job_buf);
	g_job_buf = NULL;
	g_job_len = 0;
	g_job_kind = kind;

	{
		auto (found, lok) = os_look_path(ranged(prog_path, 0, sizeof(prog_path)), str_from_cstr(prog));
		(void)found;
		if (!lok) {
			display_set_echo(str_from_cstr((char *)"[job: program not found]"));
			g_job_kind = JobNone;
			return false;
		}
		prog = prog_path;
	}
	for (i = 0; i < nargs; i++) {
		stack_args[i] = str_from_cstr(argv[i]);
	}
	c.init(str_from_cstr(prog), stack_args, nargs);
	if (!c.dir(str_from_cstr(workdir))) {
		display_set_echo(str_from_cstr((char *)"[job: bad working directory]"));
		job_clear();
		return false;
	}
	c.stdio(StdErr, IoNull);
	{
		auto (pf, ok) = c.pipe(StdOut);
		if (!ok) {
			display_set_echo(str_from_cstr((char *)"[job: pipe failed]"));
			job_clear();
			return false;
		}
		g_job_out = pf;
		g_job_out_open = true;
	}
	if (!c.start()) {
		display_set_echo(str_from_cstr((char *)"[job: start failed]"));
		job_clear();
		return false;
	}
	display_set_echo(str_from_cstr((char *)"[running...] (C-g to cancel)"));
	return true;
}

void job_init(void) {
	g_last_compile[0] = 0;
	snprintf(g_last_compile, sizeof(g_last_compile), "%s", (char *)"make -k");
	job_clear();
}

bool job_running(void) {
	return g_job_kind != JobNone;
}

bool job_cancel(void) {
	if (!job_running()) {
		return false;
	}
	g_job_cmd.close();
	free(g_job_buf);
	g_job_buf = NULL;
	g_job_len = 0;
	job_clear();
	display_set_echo(str_from_cstr((char *)"[cancelled]"));
	return true;
}

char *job_last_compile_command(void) {
	return g_last_compile;
}

bool job_start_grep(char *pattern) {
	char rg[JobPathMax] = {
		0 };
	char grep[JobPathMax] = {
		0 };
	char *argv_rg[6] = {
		0 };
	char *argv_grep[4] = {
		0 };
	if (pattern == NULL || pattern[0] == 0) {
		display_set_echo(str_from_cstr((char *)"[empty grep pattern]"));
		return false;
	}
	snprintf(g_job_arg0, sizeof(g_job_arg0), "%s", pattern);
	if (!job_project_root(g_job_root, sizeof(g_job_root))) {
		display_set_echo(str_from_cstr((char *)"[grep: no project root]"));
		return false;
	}
	{
		auto (found, rok) = os_look_path(ranged(rg, 0, sizeof(rg)), str_from_cstr((char *)"rg"));
		(void)found;
		if (rok) {
			argv_rg[0] = (char *)"-n";
			argv_rg[1] = (char *)"--column";
			argv_rg[2] = (char *)"--no-heading";
			argv_rg[3] = (char *)"--color=never";
			argv_rg[4] = g_job_arg0;
			argv_rg[5] = (char *)".";
			return job_start_argv((char *)"rg", argv_rg, 6, g_job_root, JobGrep);
		}
	}
	{
		auto (found, gok) = os_look_path(ranged(grep, 0, sizeof(grep)), str_from_cstr((char *)"grep"));
		(void)found;
		if (!gok) {
			display_set_echo(str_from_cstr((char *)"[grep: need rg or grep on PATH]"));
			return false;
		}
		argv_grep[0] = (char *)"-rn";
		argv_grep[1] = g_job_arg0;
		argv_grep[2] = (char *)".";
		return job_start_argv((char *)"grep", argv_grep, 3, g_job_root, JobGrep);
	}
}

bool job_start_compile(char *command) {
	char sh[JobPathMax] = {
		0 };
	char *argv[2] = {
		0 };
	if (command == NULL || command[0] == 0) {
		display_set_echo(str_from_cstr((char *)"[empty compile command]"));
		return false;
	}
	snprintf(g_last_compile, sizeof(g_last_compile), "%s", command);
	snprintf(g_job_arg0, sizeof(g_job_arg0), "%s", command);
	if (!job_project_root(g_job_root, sizeof(g_job_root))) {
		display_set_echo(str_from_cstr((char *)"[compile: no project root]"));
		return false;
	}
	{
		auto (found, ok) = os_look_path(ranged(sh, 0, sizeof(sh)), str_from_cstr((char *)"/bin/sh"));
		(void)found;
		if (!ok) {
			display_set_echo(str_from_cstr((char *)"[compile: /bin/sh not found]"));
			return false;
		}
		argv[0] = (char *)"-c";
		argv[1] = g_job_arg0;
		return job_start_argv(sh, argv, 2, g_job_root, JobCompile);
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
	if (buf.name == NULL) {
		return false;
	}
	if (strcmp(buf.name, "*grep*") != 0 && strcmp(buf.name, "*compile*") != 0) {
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

bool job_visit_at_point(void) {
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

void job_poll(void) {
	char chunk[4096] = {
		0 };
	Poll pol = {
		0 };
	int exit_code = 0;
	bool done = false;

	if (!job_running()) {
		return;
	}
	if (g_job_out_open) {
		for (;;) {
			auto (n, ok) = g_job_out.read(ranged(chunk, sizeof(chunk)));
			if (!ok) {
				break;
			}
			if (n == 0) {
				break;
			}
			if (!job_accum_append(chunk, (size_t)n)) {
				break;
			}
		}
	}
	pol.init();
	if (g_job_out_open && file_fd(&g_job_out) >= 0) {
		pol.add_file(&g_job_out, PollIn);
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
				Cmd *jc = pol.ready_cmd(i);
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
	job_finish(exit_code);
}
