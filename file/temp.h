#include <sys/stat.h>

int mkstemp(char *);
char *mkdtemp(char *);
int chmod(const char *path, unsigned mode);
int symlink(const char *target, const char *linkpath);
int lstat(const char *path, struct stat *buf);
int stat(const char *path, struct stat *buf);
int readlink(const char *path, char *buf, unsigned long bufsiz);
