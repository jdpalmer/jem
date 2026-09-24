import "minibuffer";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

int main() {
	minibuffer_begin("Prompt: ", 0);
	check(minibuffer_insert_paste("alpha\r\nbeta"));
	check(strcmp(minibuffer_text(), "alpha\n\nbeta") == 0);
	minibuffer_cancel();
	check(!minibuffer_insert_paste("ignored"));
	return 0;
}
