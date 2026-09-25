// runtime/vars.mc — editor variables: set-variable / describe-variable (jem.go/runtime/variables.go).
#include <stddef.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

enum {
	VarMax = 16
};

struct EditorVar {
	const char *name;
	const char *doc;
	int min_v;
	int max_v;
	bool local;
};

static EditorVar g_vars[VarMax];
static int g_var_count;

static int bool_to_int(bool b) {
	return b ? 1 : 0;
}

static void vars_theme_changed() {
	size_t i = 0;
	display_invalidate();
	for (i = 0; i < window_active.count; i++) {
		Window *win = window_active.windows[i];
		if (win != NULL) {
			win.should_redraw = true;
			win.should_update_mode_line = true;
		}
	}
}

void vars_init() {
	EditorVar *v = NULL;
	memset(g_vars, 0, sizeof(g_vars));
	g_var_count = 0;
	g_app.fill_col = 80;
	g_app.indent.width = 2;
	g_app.indent.brace = 0;
	g_app.indent.label = 0;
	g_app.indent.continued = 4;
	g_app.whitespace_cleanup = true;
	g_app.auto_indent = true;

	v = &g_vars[g_var_count++];
	v.name = "fill-column";
	v.doc = "Wrap/fill column used by paragraph filling.";
	v.min_v = 0;
	v.max_v = 1000;
	v.local = true;

	v = &g_vars[g_var_count++];
	v.name = "theme-mode";
	v.doc = "Editor palette mode: 0 dark, 1 light.";
	v.min_v = 0;
	v.max_v = 1;
	v.local = false;

	v = &g_vars[g_var_count++];
	v.name = "search-scope";
	v.doc = "Search scope: 0 current buffer, 1 all buffers.";
	v.min_v = 0;
	v.max_v = 1;
	v.local = false;

	v = &g_vars[g_var_count++];
	v.name = "whitespace-cleanup";
	v.doc = "Trim trailing whitespace from every line before saving: 0 off, 1 on.";
	v.min_v = 0;
	v.max_v = 1;
	v.local = true;

	v = &g_vars[g_var_count++];
	v.name = "auto-indent";
	v.doc = "Insert mode-aware indent after newline: 0 off, 1 on.";
	v.min_v = 0;
	v.max_v = 1;
	v.local = false;

	v = &g_vars[g_var_count++];
	v.name = "indent-width";
	v.doc = "Primary indent step in columns/spaces.";
	v.min_v = 0;
	v.max_v = 32;
	v.local = true;

	v = &g_vars[g_var_count++];
	v.name = "indent-brace";
	v.doc = "Extra indent for a standalone opening brace line (C-like modes).";
	v.min_v = 0;
	v.max_v = 32;
	v.local = true;

	v = &g_vars[g_var_count++];
	v.name = "indent-label";
	v.doc = "Extra offset applied to case/default labels (C-like modes).";
	v.min_v = 0;
	v.max_v = 32;
	v.local = true;

	v = &g_vars[g_var_count++];
	v.name = "indent-continued";
	v.doc = "Extra indent for continuation lines (Python-like modes).";
	v.min_v = 0;
	v.max_v = 32;
	v.local = true;
}

static EditorVar *var_find(const char *name) {
	int i = 0;
	if (name == NULL) {
		return NULL;
	}
	for (i = 0; i < g_var_count; i++) {
		if (strcmp(g_vars[i].name, name) == 0) {
			return &g_vars[i];
		}
	}
	return NULL;
}

static int var_read_local(EditorVar *v, Buffer *buf) {
	if (v == NULL || buf == NULL) {
		return 0;
	}
	if (strcmp(v.name, "fill-column") == 0) {
		return buf.fill_col;
	}
	if (strcmp(v.name, "whitespace-cleanup") == 0) {
		return bool_to_int(buf.whitespace_cleanup);
	}
	if (strcmp(v.name, "indent-width") == 0) {
		return buf.indent.width;
	}
	if (strcmp(v.name, "indent-brace") == 0) {
		return buf.indent.brace;
	}
	if (strcmp(v.name, "indent-label") == 0) {
		return buf.indent.label;
	}
	if (strcmp(v.name, "indent-continued") == 0) {
		return buf.indent.continued;
	}
	return 0;
}

static int var_read_global(EditorVar *v) {
	if (v == NULL) {
		return 0;
	}
	if (strcmp(v.name, "fill-column") == 0) {
		return g_app.fill_col;
	}
	if (strcmp(v.name, "theme-mode") == 0) {
		return display_theme_mode();
	}
	if (strcmp(v.name, "search-scope") == 0) {
		return search_scope_get();
	}
	if (strcmp(v.name, "whitespace-cleanup") == 0) {
		return bool_to_int(g_app.whitespace_cleanup);
	}
	if (strcmp(v.name, "auto-indent") == 0) {
		return bool_to_int(g_app.auto_indent);
	}
	if (strcmp(v.name, "indent-width") == 0) {
		return g_app.indent.width;
	}
	if (strcmp(v.name, "indent-brace") == 0) {
		return g_app.indent.brace;
	}
	if (strcmp(v.name, "indent-label") == 0) {
		return g_app.indent.label;
	}
	if (strcmp(v.name, "indent-continued") == 0) {
		return g_app.indent.continued;
	}
	return 0;
}

static int var_storage_read(EditorVar *v, Buffer *buf) {
	if (v != NULL && v.local && buf != NULL) {
		return var_read_local(v, buf);
	}
	return var_read_global(v);
}

static void var_write_local(EditorVar *v, Buffer *buf, int value) {
	if (v == NULL || buf == NULL) {
		return;
	}
	if (strcmp(v.name, "fill-column") == 0) {
		buf.fill_col = value;
		return;
	}
	if (strcmp(v.name, "whitespace-cleanup") == 0) {
		buf.whitespace_cleanup = value != 0;
		return;
	}
	if (strcmp(v.name, "indent-width") == 0) {
		buf.indent.width = value;
		return;
	}
	if (strcmp(v.name, "indent-brace") == 0) {
		buf.indent.brace = value;
		return;
	}
	if (strcmp(v.name, "indent-label") == 0) {
		buf.indent.label = value;
		return;
	}
	if (strcmp(v.name, "indent-continued") == 0) {
		buf.indent.continued = value;
		return;
	}
}

static void var_write_global(EditorVar *v, int value) {
	if (v == NULL) {
		return;
	}
	if (strcmp(v.name, "fill-column") == 0) {
		g_app.fill_col = value;
		return;
	}
	if (strcmp(v.name, "theme-mode") == 0) {
		display_set_theme_mode(value);
		return;
	}
	if (strcmp(v.name, "search-scope") == 0) {
		search_scope_set(value);
		return;
	}
	if (strcmp(v.name, "whitespace-cleanup") == 0) {
		g_app.whitespace_cleanup = value != 0;
		return;
	}
	if (strcmp(v.name, "auto-indent") == 0) {
		g_app.auto_indent = value != 0;
		return;
	}
	if (strcmp(v.name, "indent-width") == 0) {
		g_app.indent.width = value;
		return;
	}
	if (strcmp(v.name, "indent-brace") == 0) {
		g_app.indent.brace = value;
		return;
	}
	if (strcmp(v.name, "indent-label") == 0) {
		g_app.indent.label = value;
		return;
	}
	if (strcmp(v.name, "indent-continued") == 0) {
		g_app.indent.continued = value;
		return;
	}
}

static bool var_storage_write(EditorVar *v, Buffer *buf, int value, bool run_on_change) {
	if (v == NULL || value < v.min_v || value > v.max_v) {
		return false;
	}
	if (v.local && buf != NULL) {
		var_write_local(v, buf, value);
	} else {
		var_write_global(v, value);
	}
	if (run_on_change && strcmp(v.name, "theme-mode") == 0) {
		vars_theme_changed();
	}
	if (run_on_change && strcmp(v.name, "search-scope") == 0) {
		size_t i = 0;
		for (i = 0; i < window_active.count; i++) {
			Window *win = window_active.windows[i];
			if (win != NULL) {
				win.should_update_mode_line = true;
			}
		}
	}
	return true;
}

static bool parse_numeric_text(const char *text, int *out) {
	char *end = NULL;
	int64_t n = 0;
	if (text == NULL || out == NULL) {
		return false;
	}
	while (*text == ' ' || *text == '\t') {
		text++;
	}
	if (*text == 0) {
		return false;
	}
	if (text[0] == '0' && (text[1] == 'x' || text[1] == 'X')) {
		n = strtoll(text + 2, &end, 16);
	} else {
		n = strtoll(text, &end, 10);
	}
	if (end == text || n < 0 || n > 2147483647) {
		return false;
	}
	while (*end == ' ' || *end == '\t') {
		end++;
	}
	if (*end != 0) {
		return false;
	}
	*out = (int)n;
	return true;
}

static void var_fuzzy_open(const char *prompt) {
	const char *names[VarMax] = {0};
	int i = 0;
	for (i = 0; i < g_var_count; i++) {
		names[i] = g_vars[i].name;
	}
	fuzzy_command_open(names, NULL, g_var_count, prompt, NULL);
}

// cmd_set_variable prompts for a variable name then a numeric value.
bool cmd_set_variable(bool f, int n) {
	(void)f;
	(void)n;
	if (g_var_count == 0) {
		vars_init();
	}
	push_listener(ListenerVarFuzzy);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = VarFuzzySet;
		}
	}
	var_fuzzy_open("Set variable: ");
	return true;
}

// cmd_describe_variable shows a variable's value and documentation.
bool cmd_describe_variable(bool f, int n) {
	(void)f;
	(void)n;
	if (g_var_count == 0) {
		vars_init();
	}
	push_listener(ListenerVarFuzzy);
	{
		Listener *L = top_listener();
		if (L != NULL) {
			L.ask_phase = VarFuzzyDescribe;
		}
	}
	var_fuzzy_open("Describe variable: ");
	return true;
}

bool cmd_var_fuzzy_on_accept() {
	char *sel = fuzzy_command_selected();
	char name[64] = {0};
	int phase = VarFuzzySet;
	Listener *L = top_listener();
	EditorVar *v = NULL;
	Buffer *buf = buffer_current();
	char msg[256] = {0};
	if (L != NULL) {
		phase = L.ask_phase;
	}
	if (sel != NULL) {
		snprintf(name, sizeof(name), "%s", sel);
	}
	fuzzy_command_close();
	pop_listener();
	v = var_find(name);
	if (v == NULL) {
		display_set_echo("[unknown variable]");
		return true;
	}
	if (phase == (int)VarFuzzyDescribe) {
		snprintf(msg, sizeof(msg), "[%s = %d] %s", v.name, var_storage_read(v, buf), v.doc);
		display_set_echo(msg);
		return true;
	}
	/* VarFuzzySet — ask for value */
	push_listener(ListenerAskString);
	{
		Listener *N = top_listener();
		char prompt[96] = {0};
		if (N != NULL) {
			N.ask_phase = AskSetVariableValue;
			snprintf(N.ask_buf, sizeof(N.ask_buf), "%s", v.name);
		}
		snprintf(prompt, sizeof(prompt), "Set %s (current %d): ", v.name, var_storage_read(v, buf));
		minibuffer_begin(str_from_cstr(prompt), 0);
	}
	return true;
}

bool cmd_var_fuzzy_on_abort() {
	fuzzy_command_close();
	pop_listener();
	display_set_echo("Quit");
	return true;
}

bool cmd_set_variable_value_from_text(const char *var_name, const char *text) {
	EditorVar *v = var_find(var_name);
	Buffer *buf = buffer_current();
	int parsed = 0;
	char msg[128] = {0};
	if (v == NULL) {
		display_set_echo("[unknown variable]");
		return false;
	}
	if (!parse_numeric_text(text, &parsed) ||
		!var_storage_write(v, buf, parsed, !v.local)) {
		snprintf(msg, sizeof(msg), "[invalid value for %s]", v.name);
		display_set_echo(msg);
		return false;
	}
	snprintf(msg, sizeof(msg), "[%s = %d]", v.name, var_storage_read(v, buf));
	display_set_echo(msg);
	return true;
}

/* Test helpers */
int vars_count() {
	if (g_var_count == 0) {
		vars_init();
	}
	return g_var_count;
}

bool vars_set_named(const char *name, int value) {
	EditorVar *v = NULL;
	if (g_var_count == 0) {
		vars_init();
	}
	v = var_find(name);
	if (v == NULL) {
		return false;
	}
	return var_storage_write(v, buffer_current(), value, !v.local);
}

int vars_get_named(const char *name) {
	EditorVar *v = NULL;
	if (g_var_count == 0) {
		vars_init();
	}
	v = var_find(name);
	if (v == NULL) {
		return 0;
	}
	return var_storage_read(v, buffer_current());
}
