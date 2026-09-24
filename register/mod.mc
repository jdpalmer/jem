// register — named text registers (a-z).
import "str";

#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

struct Reg {
	char[..] data; /* owned heap */
};

static Reg g_regs[26];

static int reg_index(char name) {
	if (name >= 'a' && name <= 'z') {
		return name - 'a';
	}
	if (name >= 'A' && name <= 'Z') {
		return name - 'A';
	}
	return -1;
}

bool register_set(char name, const char[..] src) {
	int i = reg_index(name);
	if (i < 0) {
		return false;
	}
	return str_set(&g_regs[i].data, src);
}

/* Borrow of register contents (owned by register). */
char[..] register_get(char name) {
	int i = reg_index(name);
	if (i < 0) {
		return str_empty();
	}
	return g_regs[i].data;
}
