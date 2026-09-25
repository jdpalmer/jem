/*
 * Host calendar time for %C — prototypes only; link against libc.
 * (modc's curated <time.h> omits time/localtime.)
 */
#pragma once

#include <stdint.h>

struct JemTm {
	int tm_sec;
	int tm_min;
	int tm_hour;
	int tm_mday;
	int tm_mon;
	int tm_year;
	int tm_wday;
	int tm_yday;
	int tm_isdst;
	int64_t tm_gmtoff;
	char *tm_zone;
};

/* Darwin/Linux: time_t is a signed word-sized integer. */
int64_t time(int64_t *tloc);
struct JemTm *localtime(const int64_t *timer);
