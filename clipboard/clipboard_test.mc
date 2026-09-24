import "clipboard";

#include <stdbool.h>
#include <stdlib.h>

static void check(bool value) {
	if (!value) {
		abort();
	}
}

int main() {
	bool initialized = clipboard_init();
	check(clipboard_native_ready() == initialized);
	check(clipboard_init() == initialized);
	return 0;
}
