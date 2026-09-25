// display/git.mc — git modeline text + gutter diff markers (jem.go/display/git.go, jem.c/src/git.c).
import "buffer";
import "os";
import "path";
import "str";

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include "../runtime/host_time.h"

enum {
	GitCacheMax = 32,
	GitPathMax = 512,
	GitTextMax = 96,
	GitStatusCap = 16384,
	GitDiffCapMax = 1048576
};

enum GitLineDiff {
	GitLineDiffNone = 0,
	GitLineDiffAdded = 1,
	GitLineDiffModified = 2,
	GitLineDiffDeleted = 3
};

struct GitModelineCache {
	Buffer *buffer;
	bool valid;
	bool has_repo;
	int64_t refreshed_at;
	char file_name[GitPathMax];
	char text[GitTextMax];
	uint8_t *line_diffs;
	size_t diff_count;
};

static GitModelineCache g_git_caches[GitCacheMax];
static char g_git_bin[GitPathMax];
static bool g_git_bin_looked;

static const char *git_prog() {
	if (!g_git_bin_looked) {
		g_git_bin_looked = true;
		g_git_bin[0] = 0;
		{
			auto (found, ok) = os_look_path(ranged(g_git_bin, 0, sizeof(g_git_bin)), "git");
			(void)found;
			if (!ok) {
				g_git_bin[0] = 0;
			}
		}
	}
	if (g_git_bin[0] == 0) {
		return NULL;
	}
	return g_git_bin;
}

/* Runs git with argv (excluding program). Returns false if exec failed. */
static bool git_run(const char[..] args[], size_t nargs, char *out, size_t out_cap, int *exit_out) {
	Cmd c = {0};
	const char *prog = git_prog();
	size_t got = 0;
	File pf = {0};
	bool pipe_ok = false;
	if (prog == NULL || out == NULL || out_cap < 2 || exit_out == NULL) {
		return false;
	}
	out[0] = 0;
	*exit_out = -1;
	c.init(str_from_cstr(prog), args, nargs);
	{
		auto (f, ok) = c.pipe(StdOut);
		if (!ok) {
			c.close();
			return false;
		}
		pf = f;
		pipe_ok = true;
	}
	(void)c.stdio(StdErr, IoNull);
	if (!c.start()) {
		c.close();
		return false;
	}
	while (pipe_ok && got + 1 < out_cap) {
		auto (n, ok) = pf.read(ranged(out + got, out_cap - 1 - got));
		if (!ok || n == 0) {
			break;
		}
		got += n;
	}
	out[got] = 0;
	{
		auto (code, ok) = c.wait();
		if (ok) {
			*exit_out = code;
		}
	}
	c.close();
	return true;
}

static const char *git_next_line(const char *next, const char *end, const char **line_end_out) {
	const char *line_end = next;
	while (line_end < end && *line_end != '\n' && *line_end != '\r') {
		line_end++;
	}
	*line_end_out = line_end;
	while (line_end < end && (*line_end == '\n' || *line_end == '\r')) {
		line_end++;
	}
	return line_end;
}

static void git_set_line_diff(GitModelineCache *cache, size_t line_number, int marker) {
	uint8_t *slot = NULL;
	if (cache == NULL || line_number == 0 || line_number > cache.diff_count) {
		return;
	}
	slot = &cache.line_diffs[line_number - 1];
	if ((uint8_t)marker > *slot) {
		*slot = (uint8_t)marker;
	}
}

bool git_parse_hunk_header(const char *line, int *old_start, int *old_count,
	int *new_start, int *new_count) {
	if (line == NULL || old_start == NULL || old_count == NULL || new_start == NULL || new_count == NULL) {
		return false;
	}
	*old_count = 1;
	*new_count = 1;
	if (sscanf(line, "@@ -%d,%d +%d,%d @@", old_start, old_count, new_start, new_count) == 4) {
		return true;
	}
	if (sscanf(line, "@@ -%d +%d,%d @@", old_start, new_start, new_count) == 3) {
		*old_count = 1;
		return true;
	}
	if (sscanf(line, "@@ -%d,%d +%d @@", old_start, old_count, new_start) == 3) {
		*new_count = 1;
		return true;
	}
	if (sscanf(line, "@@ -%d +%d @@", old_start, new_start) == 2) {
		*old_count = 1;
		*new_count = 1;
		return true;
	}
	return false;
}

static void git_refresh_cache(GitModelineCache *cache, Buffer *buf, const char *fname, int64_t now) {
	char dir[GitPathMax] = {0};
	char basename[256] = {0};
	char status_out[GitStatusCap] = {0};
	char *diff_out = NULL;
	size_t diff_cap = 0;
	int status_exit = -1;
	int diff_exit = -1;
	char branch[64] = {0};
	char oid[40] = {0};
	int ahead = 0;
	int behind = 0;
	bool file_untracked = false;
	char file_index_status = '.';
	char file_worktree_status = '.';
	const char[..] args[8] = {0};
	size_t i = 0;

	cache.valid = true;
	cache.has_repo = false;
	cache.refreshed_at = now;
	snprintf(cache.file_name, sizeof(cache.file_name), "%s", fname);
	cache.text[0] = 0;
	free(cache.line_diffs);
	cache.line_diffs = NULL;
	cache.diff_count = 0;
	if (buf.line_count_i() > 0) {
		cache.diff_count = buf.line_count_i();
		cache.line_diffs = (uint8_t *)calloc(cache.diff_count, 1);
		if (cache.line_diffs == NULL) {
			cache.diff_count = 0;
			return;
		}
	}

	{
		char[..] fv = str_from_cstr(fname);
		auto dv = path_dir(fv);
		auto bv = path_base(fv);
		if (len(dv) == 0) {
			snprintf(dir, sizeof(dir), ".");
		} else {
			snprintf(dir, sizeof(dir), "%.*s", (int)len(dv), ptr(dv));
		}
		if (len(bv) == 0) {
			return;
		}
		snprintf(basename, sizeof(basename), "%.*s", (int)len(bv), ptr(bv));
	}

	args[0] = "-C";
	args[1] = str_from_cstr(dir);
	args[2] = "status";
	args[3] = "--branch";
	args[4] = "--porcelain=2";
	args[5] = "--untracked-files=all";
	args[6] = "--";
	args[7] = str_from_cstr(basename);
	if (!git_run(args, 8, status_out, sizeof(status_out), &status_exit) || status_exit != 0) {
		return;
	}

	{
		const char *next = status_out;
		const char *end = status_out + strlen(status_out);
		while (next < end) {
			const char *line_end = NULL;
			const char *line_start = next;
			char line[256] = {0};
			size_t line_len = 0;
			next = git_next_line(next, end, &line_end);
			line_len = line_end - line_start;
			if (line_len == 0) {
				continue;
			}
			if (line_len >= sizeof(line)) {
				line_len = sizeof(line) - 1;
			}
			memcpy(line, line_start, line_len);
			line[line_len] = 0;
			if (strncmp(line, "# branch.head ", 14) == 0) {
				snprintf(branch, sizeof(branch), "%s", line + 14);
			} else if (strncmp(line, "# branch.oid ", 13) == 0) {
				snprintf(oid, sizeof(oid), "%s", line + 13);
			} else if (strncmp(line, "# branch.ab ", 12) == 0) {
				(void)sscanf(line + 12, "+%d -%d", &ahead, &behind);
			} else if ((line[0] == '1' || line[0] == '2') && line[1] == ' ') {
				file_index_status = line[2];
				file_worktree_status = line[3];
			} else if (line[0] == '?') {
				file_untracked = true;
			}
		}
	}

	cache.has_repo = true;
	{
		char head[64] = {0};
		char dirty[8] = {0};
		char ahead_part[16] = {0};
		char behind_part[16] = {0};
		char short_oid[16] = {0};
		snprintf(short_oid, sizeof(short_oid), "%.7s", oid);
		if (strcmp(branch, "(detached)") == 0) {
			snprintf(head, sizeof(head), "@%s", short_oid);
		} else if (branch[0] != 0) {
			snprintf(head, sizeof(head), "%s", branch);
		} else if (oid[0] != 0) {
			snprintf(head, sizeof(head), "@%s", short_oid);
		} else {
			snprintf(head, sizeof(head), "git");
		}
		if (file_untracked) {
			snprintf(dirty, sizeof(dirty), "[?]");
			for (i = 0; i < cache.diff_count; i++) {
				cache.line_diffs[i] = (uint8_t)GitLineDiffAdded;
			}
		} else if (file_index_status != '.' || file_worktree_status != '.') {
			snprintf(dirty, sizeof(dirty), "[%c%c]", file_index_status, file_worktree_status);
		}
		if (ahead > 0) {
			snprintf(ahead_part, sizeof(ahead_part), "^%d", ahead);
		}
		if (behind > 0) {
			snprintf(behind_part, sizeof(behind_part), "v%d", behind);
		}
		snprintf(cache.text, sizeof(cache.text), "%s%s%s%s", head, dirty, ahead_part, behind_part);
	}

	if (file_untracked) {
		return;
	}

	diff_cap = cache.diff_count * 48 + 4096;
	if (diff_cap < 16384) {
		diff_cap = 16384;
	} else if (diff_cap > (size_t)GitDiffCapMax) {
		diff_cap = GitDiffCapMax;
	}
	diff_out = (char *)malloc(diff_cap);
	if (diff_out == NULL) {
		return;
	}
	diff_out[0] = 0;
	{
		const char[..] dargs[9] = {0};
		dargs[0] = "-C";
		dargs[1] = str_from_cstr(dir);
		dargs[2] = "diff";
		dargs[3] = "--no-color";
		dargs[4] = "--no-ext-diff";
		dargs[5] = "--unified=0";
		dargs[6] = "HEAD";
		dargs[7] = "--";
		dargs[8] = str_from_cstr(basename);
		if (!git_run(dargs, 9, diff_out, diff_cap, &diff_exit) || diff_exit != 0) {
			free(diff_out);
			return;
		}
	}

	{
		const char *next = diff_out;
		const char *end = diff_out + strlen(diff_out);
		while (next < end) {
			const char *line_end = NULL;
			const char *line_start = next;
			char line[256] = {0};
			size_t line_len = 0;
			int old_start = 0;
			int old_count = 1;
			int new_start = 0;
			int new_count = 1;
			next = git_next_line(next, end, &line_end);
			line_len = line_end - line_start;
			if (line_len < 3 || line_start[0] != '@' || line_start[1] != '@') {
				continue;
			}
			if (line_len >= sizeof(line)) {
				line_len = sizeof(line) - 1;
			}
			memcpy(line, line_start, line_len);
			line[line_len] = 0;
			if (!git_parse_hunk_header(line, &old_start, &old_count, &new_start, &new_count)) {
				continue;
			}
			(void)old_start;
			if (old_count == 0 && new_count > 0) {
				int k = 0;
				for (k = 0; k < new_count; k++) {
					int line_number = new_start + k;
					if (line_number > 0) {
						git_set_line_diff(cache, line_number, GitLineDiffAdded);
					}
				}
				continue;
			}
			if (new_count == 0 && old_count > 0) {
				size_t target = new_start;
				if (target == 0) {
					target = 1;
				}
				if (cache.diff_count > 0 && target > cache.diff_count) {
					target = cache.diff_count;
				}
				git_set_line_diff(cache, target, GitLineDiffDeleted);
				continue;
			}
			{
				int k = 0;
				for (k = 0; k < new_count; k++) {
					int line_number = new_start + k;
					if (line_number > 0) {
						git_set_line_diff(cache, line_number, GitLineDiffModified);
					}
				}
				if (old_count > new_count) {
					int line_number = new_start + new_count;
					if (line_number > 0) {
						git_set_line_diff(cache, line_number, GitLineDiffDeleted);
					}
				}
			}
		}
	}
	free(diff_out);
}

static GitModelineCache *git_cache_for_buffer(Buffer *buf) {
	GitModelineCache *cache = NULL;
	GitModelineCache *free_slot = NULL;
	size_t i = 0;
	int64_t now = 0;
	char fname[GitPathMax] = {0};
	if (buf == NULL || len(buf.file_name) == 0) {
		return NULL;
	}
	snprintf(fname, sizeof(fname), "%.*s", (int)len(buf.file_name), ptr(buf.file_name));
	if (fname[0] == 0) {
		return NULL;
	}
	for (i = 0; i < (size_t)GitCacheMax; i++) {
		if (g_git_caches[i].buffer == buf) {
			cache = &g_git_caches[i];
			break;
		}
		if (free_slot == NULL && g_git_caches[i].buffer == NULL) {
			free_slot = &g_git_caches[i];
		}
	}
	if (cache == NULL) {
		cache = free_slot;
	}
	if (cache == NULL) {
		/* reuse first slot if full */
		cache = &g_git_caches[0];
		free(cache.line_diffs);
		memset(cache, 0, sizeof(*cache));
	}
	if (cache.buffer == NULL) {
		cache.buffer = buf;
		cache.valid = false;
		cache.has_repo = false;
		cache.refreshed_at = 0;
		cache.file_name[0] = 0;
		cache.text[0] = 0;
		cache.line_diffs = NULL;
		cache.diff_count = 0;
	}
	now = time(NULL);
	if (!cache.valid || strcmp(cache.file_name, fname) != 0 || cache.refreshed_at != now) {
		git_refresh_cache(cache, buf, fname, now);
	}
	return cache;
}

// git_modeline_text returns branch/status for the modeline, or "" when unavailable.
const char *git_modeline_text(Buffer *buf) {
	GitModelineCache *cache = git_cache_for_buffer(buf);
	if (cache == NULL || !cache.has_repo || cache.text[0] == 0) {
		return "";
	}
	return cache.text;
}

// git_line_diff returns the gutter marker for a 1-based line number.
int git_line_diff(Buffer *buf, int line_number) {
	GitModelineCache *cache = git_cache_for_buffer(buf);
	if (cache == NULL || !cache.has_repo || line_number <= 0 ||
		(size_t)line_number > cache.diff_count) {
		return GitLineDiffNone;
	}
	return cache.line_diffs[line_number - 1];
}

void git_forget_buffer(Buffer *buf) {
	size_t i = 0;
	if (buf == NULL) {
		return;
	}
	for (i = 0; i < (size_t)GitCacheMax; i++) {
		if (g_git_caches[i].buffer == buf) {
			free(g_git_caches[i].line_diffs);
			memset(&g_git_caches[i], 0, sizeof(g_git_caches[i]));
			return;
		}
	}
}
