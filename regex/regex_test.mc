import "regex";

#include <stdbool.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>

static void check(bool ok) {
	if (!ok) {
		abort();
	}
}

static void test_find_simple() {
	ReCapture caps[ReMaxCaptures] = {0};
	auto (st, off, n) = re_find("a+", "xxaaayy");
	check(st == ReFound);
	check(off == 2);
	check(n == 3);
	(void)caps;
}

static void test_find_groups() {
	ReCapture caps[ReMaxCaptures] = {0};
	auto (st, off, n) = re_find_groups("(a)(b)", "zab", ranged(caps, ReMaxCaptures));
	check(st == ReFound);
	check(off == 1);
	check(n == 2);
	check(caps[0].start == 1);
	check(caps[0].length == 2);
	check(caps[1].start == 1);
	check(caps[1].length == 1);
	check(caps[2].start == 2);
	check(caps[2].length == 1);
}

static void test_invalid() {
	auto (st, off, n) = re_find("[", "abc");
	(void)off;
	(void)n;
	check(st == ReInvalid);
}

static void test_none() {
	auto (st, off, n) = re_find("zzz", "abc");
	(void)off;
	(void)n;
	check(st == ReNone);
}

static void test_dot_newline() {
	char text[] = {'a', '\n', 'b'};
	auto (st, off, n) = re_find("a.b", ranged(text, 3));
	check(st == ReFound);
	check(off == 0);
	check(n == 3);
}

static void test_classes() {
	auto (st, off, n) = re_find("\\d+", "ab12cd");
	check(st == ReFound);
	check(off == 2);
	check(n == 2);
}

int main() {
	test_find_simple();
	test_find_groups();
	test_invalid();
	test_none();
	test_dot_newline();
	test_classes();
	return 0;
}
