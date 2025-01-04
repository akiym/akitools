/* gcc -m32 -fno-stack-protector -zexecstack shellcode.c */

unsigned char shellcode[4096];

int main(void) {
    read(0, shellcode, 4096);
    (*(void(*)()) shellcode)();
    return 0;
}
