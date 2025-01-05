#include <unistd.h>
#include <sys/mman.h>

int main(void) {
    unsigned char *shellcode = mmap((void *)0x20000000, 4096, PROT_READ|PROT_WRITE|PROT_EXEC, MAP_PRIVATE|MAP_ANONYMOUS, -1, 0);
    read(0, shellcode, 4096);
    (*(void(*)()) shellcode)();
    return 0;
}
