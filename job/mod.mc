// job — background compile/grep via os.Cmd + Poll (non-blocking main loop).
import "buffer";
import "display";
import "window";
import "fs";
import "os";
import "path";
import "str";

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>

enum {
	JobOutCap = 1024 * 1024,
	JobPathMax = 4096 };

enum JobKind {
	JobNone = 0,
	JobGrep = 1,
	JobCompile = 2 };

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

static Buffer *job_show_buffer(char *name, char *data, size_t n) {
	Buffer *buf = buffer_find_by_name(name);
	if (buf == NULL) {
		buf = buffer_new();
		if (buf == NULL) {
			return NULL;
		}
		buf.set_name(name);
		if (!buffer_list_add(buf)) {
			buf.free();
			return NULL;
		}
	}
	buf.load_bytes(data, n, EModeLF);
	buf.is_changed = false;
	buffer_set_current(buf);
	window_switch_buffer(buf);
	return buf;
}

static void job_finish(int exit_code) {
	char header[256] = {
		0 };
	if (g_job_kind == JobGrep) {
		(void)job_show_buffer((char *)"*grep*", g_job_buf, g_job_len);
		snprintf(header, sizeof(header), "[grep done exit=%d]", exit_code);
	} else if (g_job_kind == JobCompile) {
		(void)job_show_buffer((char *)"*compile*", g_job_buf, g_job_len);
		snprintf(header, sizeof(header), "[compile done exit=%d]", exit_code);
	} else {
		snprintf(header, sizeof(header), "[job done exit=%d]", exit_code);
	}
	display_set_echo(str_from_cstr(header));
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
	char *argv_rg[5] = {
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
		{
			auto (found, rok) = os_look_path(ranged(rg, 0, sizeof(rg)), str_from_cstr((char *)"rg"));
			(void)found;
			if (rok) {
			argv_rg[0] = (char *)"-n";
			argv_rg[1] = (char *)"--no-heading";
			argv_rg[2] = (char *)"--color=never";
			argv_rg[3] = g_job_arg0;
			argv_rg[4] = (char *)".";
				return job_start_argv((char *)"rg", argv_rg, 5, g_job_root, JobGrep);
			}
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
