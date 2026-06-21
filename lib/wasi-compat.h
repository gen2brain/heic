#pragma once

/* wasi-libc has no mkstemp; libheif references it on a path the decoder never
 * reaches. Declare it here so it compiles, stub it in heif.c. */
int mkstemp(char *);
