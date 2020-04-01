// +build darwin linux,!baremetal,!wasi freebsd,!baremetal
// +build !nintendoswitch

package runtime

import (
	"unsafe"
)

//export putchar
func _putchar(c int) int

//export usleep
func usleep(usec uint) int

//export malloc
func malloc(size uintptr) unsafe.Pointer

// void *mmap(void *addr, size_t length, int prot, int flags, int fd, off_t offset);
// Note: off_t is defined as int64 because:
//   - musl (used on Linux) always defines it as int64
//   - darwin is practically always 64-bit anyway
//export mmap
func mmap(addr unsafe.Pointer, length uintptr, prot, flags, fd int, offset int64) unsafe.Pointer

//export abort
func abort()

//export exit
func exit(code int)

//export clock_gettime
func libc_clock_gettime(clk_id int32, ts *timespec)

//export __clock_gettime64
func libc_clock_gettime64(clk_id int32, ts *timespec)

// Portable (64-bit) variant of clock_gettime.
func clock_gettime(clk_id int32, ts *timespec) {
	if TargetBits == 32 {
		// This is a 32-bit architecture (386, arm, etc).
		// We would like to use the 64-bit version of this function so that
		// binaries will continue to run after Y2038.
		// For more information:
		//   - https://musl.libc.org/time64.html
		//   - https://sourceware.org/glibc/wiki/Y2038ProofnessDesign
		libc_clock_gettime64(clk_id, ts)
	} else {
		// This is a 64-bit architecture (amd64, arm64, etc).
		// Use the regular variant, because it already fixes the Y2038 problem
		// by using 64-bit integer types.
		libc_clock_gettime(clk_id, ts)
	}
}

type timeUnit int64

// Note: tv_sec and tv_nsec normally vary in size by platform. However, we're
// using the time64 variant (see clock_gettime above), so the formats are the
// same between 32-bit and 64-bit architectures.
// There is one issue though: on big-endian systems, tv_nsec would be incorrect.
// But we don't support big-endian systems yet (as of 2021) so this is fine.
type timespec struct {
	tv_sec  int64 // time_t with time64 support (always 64-bit)
	tv_nsec int64 // unsigned 64-bit integer on all time64 platforms
}

var stackTop uintptr

func postinit() {}

// Entry point for Go. Initialize all packages and call main.main().
//export main
func main(argc int32, argv *unsafe.Pointer) int {
	preinit()

	// Make args global big enough so that it can store all command line
	// arguments. Unfortunately this has to be done with some magic as the heap
	// is not yet initialized.
	argsSlice := (*struct {
		ptr unsafe.Pointer
		len uintptr
		cap uintptr
	})(unsafe.Pointer(&args))
	argsSlice.ptr = malloc(uintptr(argc) * (unsafe.Sizeof(uintptr(0))) * 3)
	argsSlice.len = uintptr(argc)
	argsSlice.cap = uintptr(argc)

	// Initialize command line parameters.
	for i := 0; i < int(argc); i++ {
		// Convert the C string to a Go string.
		length := strlen(*argv)
		arg := (*_string)(unsafe.Pointer(&args[i]))
		arg.length = length
		arg.ptr = (*byte)(*argv)
		// This is the Go equivalent of "argc++" in C.
		argv = (*unsafe.Pointer)(unsafe.Pointer(uintptr(unsafe.Pointer(argv)) + unsafe.Sizeof(argv)))
	}

	// Obtain the initial stack pointer right before calling the run() function.
	// The run function has been moved to a separate (non-inlined) function so
	// that the correct stack pointer is read.
	stackTop = getCurrentStackPointer()
	runMain()

	// For libc compatibility.
	return 0
}

// Must be a separate function to get the correct stack pointer.
//go:noinline
func runMain() {
	run()
}

//go:extern environ
var environ *unsafe.Pointer

//go:linkname syscall_runtime_envs syscall.runtime_envs
func syscall_runtime_envs() []string {
	// Count how many environment variables there are.
	env := environ
	numEnvs := 0
	for *env != nil {
		numEnvs++
		env = (*unsafe.Pointer)(unsafe.Pointer(uintptr(unsafe.Pointer(env)) + unsafe.Sizeof(environ)))
	}

	// Create a string slice of all environment variables.
	// This requires just a single heap allocation.
	env = environ
	envs := make([]string, 0, numEnvs)
	for *env != nil {
		ptr := *env
		length := strlen(ptr)
		s := _string{
			ptr:    (*byte)(ptr),
			length: length,
		}
		envs = append(envs, *(*string)(unsafe.Pointer(&s)))
		env = (*unsafe.Pointer)(unsafe.Pointer(uintptr(unsafe.Pointer(env)) + unsafe.Sizeof(environ)))
	}

	return envs
}

func putchar(c byte) {
	_putchar(int(c))
}

func ticksToNanoseconds(ticks timeUnit) int64 {
	// The OS API works in nanoseconds so no conversion necessary.
	return int64(ticks)
}

func nanosecondsToTicks(ns int64) timeUnit {
	// The OS API works in nanoseconds so no conversion necessary.
	return timeUnit(ns)
}

func sleepTicks(d timeUnit) {
	// timeUnit is in nanoseconds, so need to convert to microseconds here.
	usleep(uint(d) / 1000)
}

func getTime(clock int32) uint64 {
	ts := timespec{}
	clock_gettime(clock, &ts)
	return uint64(ts.tv_sec)*1000*1000*1000 + uint64(ts.tv_nsec)
}

// Return monotonic time in nanoseconds.
func monotime() uint64 {
	return getTime(clock_MONOTONIC_RAW)
}

func ticks() timeUnit {
	return timeUnit(monotime())
}

//go:linkname now time.now
func now() (sec int64, nsec int32, mono int64) {
	ts := timespec{}
	clock_gettime(clock_REALTIME, &ts)
	sec = int64(ts.tv_sec)
	nsec = int32(ts.tv_nsec)
	mono = nanotime()
	return
}

//go:linkname syscall_Exit syscall.Exit
func syscall_Exit(code int) {
	exit(code)
}

func extalloc(size uintptr) unsafe.Pointer {
	return malloc(size)
}

//export free
func extfree(ptr unsafe.Pointer)

// TinyGo does not yet support any form of parallelism on an OS, so these can be
// left empty.

//go:linkname procPin sync/atomic.runtime_procPin
func procPin() {
}

//go:linkname procUnpin sync/atomic.runtime_procUnpin
func procUnpin() {
}
