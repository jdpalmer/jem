import "file";
import "fs";
import "os";
import "path";
import "str";

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <unistd.h>
#include "temp.h"

static void check(bool ok) {
	if (!ok) {
		abort();
	}
}

int main() {
	char dir[512] = {0};
	char path[512] = {0};
	char buf[512] = {0};
	int n = 0;

	n = snprintf(dir, sizeof(dir), "/tmp/jem-fcomp-XXXXXX");
	check(n > 0);
	check(mkdtemp(dir) != NULL);

	snprintf(path, sizeof(path), "%s/alpha.txt", dir);
	{
		auto (w, wok) = writer_open(str_from_cstr(path), OpenWrite | OpenCreate | OpenTrunc, 0);
		check(wok);
		(void)w.write(str_from_cstr("hi"));
		w.close();
	}
	snprintf(path, sizeof(path), "%s/alphabet.go", dir);
	{
		auto (w, wok) = writer_open(str_from_cstr(path), OpenWrite | OpenCreate | OpenTrunc, 0);
		check(wok);
		w.close();
	}
	snprintf(path, sizeof(path), "%s/subdir", dir);
	check(fs_mkdir(str_from_cstr(path)));

	snprintf(buf, sizeof(buf), "%s/alp", dir);
	check(file_complete_path(buf, sizeof(buf)));
	check(strstr(buf, "/alpha") != NULL);

	snprintf(buf, sizeof(buf), "%s/sub", dir);
	check(file_complete_path(buf, sizeof(buf)));
	check(strlen(buf) > 0 && buf[strlen(buf) - 1] == '/');

	snprintf(buf, sizeof(buf), "~");
	check(file_complete_path(buf, sizeof(buf)));
	check(strcmp(buf, "~/") == 0);

	return 0;
}
