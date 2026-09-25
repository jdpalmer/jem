import "minibuffer";
import "str";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

static void test_paste() {
	minibuffer_begin("Prompt: ", 0);
	check(minibuffer_insert_paste("alpha\r\nbeta"));
	check(strcmp(minibuffer_text(), "alpha\n\nbeta") == 0);
	minibuffer_cancel();
	check(!minibuffer_insert_paste("ignored"));
}

static void test_history() {
	check(minibuffer_history_count() == 0);
	minibuffer_history_add("alpha");
	minibuffer_history_add("alpha"); /* dup of latest ignored */
	check(minibuffer_history_count() == 1);
	minibuffer_history_add("beta");
	check(minibuffer_history_count() == 2);
	check(strcmp(minibuffer_history_at(0), "beta") == 0);
	check(strcmp(minibuffer_history_at(1), "alpha") == 0);

	minibuffer_begin("! ", 0);
	minibuffer_set_text("draft");
	check(minibuffer_step_history(-1));
	check(strcmp(minibuffer_text(), "beta") == 0);
	check(minibuffer_step_history(-1));
	check(strcmp(minibuffer_text(), "alpha") == 0);
	check(!minibuffer_step_history(-1));
	check(minibuffer_step_history(1));
	check(strcmp(minibuffer_text(), "beta") == 0);
	check(minibuffer_step_history(1));
	check(strcmp(minibuffer_text(), "draft") == 0);
	check(!minibuffer_step_history(1));
	minibuffer_cancel();
}

int main() {
	test_paste();
	test_history();
	return 0;
}
