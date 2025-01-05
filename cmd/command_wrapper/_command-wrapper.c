#include <stdlib.h>
#include <unistd.h>

int main(int argc, char *argv[]) {
    if (argc < 1) {
        return 1;
    }
    char *envvar = getenv("COMMAND_WRAPPER_ENV");
    if (envvar != NULL) {
        char *env[] = {envvar, NULL};
        return execve(argv[1], argv + 1, env);
    } else {
        return execve(argv[1], argv + 1, NULL);
    }
}
