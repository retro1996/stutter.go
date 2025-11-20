#include <trim.h>

#if defined(__linux__) && defined(__GLIBC__)
    #include <malloc.h>
    void Trim() {
        malloc_trim(0);
    }
#else
    void Trim() {
        // Mac 或其他系统不需要/不支持手动 Trim，留空即可
    }
#endif
