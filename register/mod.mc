// register — named text registers (a-z).
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

struct Reg {
	char *data;
	size_t len;
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

bool register_set(char name, char *data, size_t n) {
	int i = reg_index(name);
	Reg *r = {0};
	if (i < 0) {
		return false;
	}
	r = &g_regs[i];
	free(r.data);
	r.data = NULL;
	r.len = 0;
	if (n == 0) {
		return true;
	}
	r.data = malloc(n + 1);
	if (r.data == NULL) {
		return false;
	}
	memcpy(r.data, data, n);
	r.data[n] = 0;
	r.len = n;
	return true;
}

(char *, size_t) register_get(char name) {
	int i = reg_index(name);
	if (i < 0) {
		return (NULL, 0);
	}
	return (g_regs[i].data, g_regs[i].len);
}
