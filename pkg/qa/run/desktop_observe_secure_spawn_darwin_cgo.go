//go:build darwin && cgo

package run

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security -lproc
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <errno.h>
#include <fcntl.h>
#include <libproc.h>
#include <signal.h>
#include <spawn.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <unistd.h>

extern char **environ;

typedef struct {
  int pid;
  int stdin_fd;
  int stdout_fd;
  int stderr_fd;
} q12_spawn_result;

static void q12_close(int *fd) {
  if (*fd >= 0) { close(*fd); *fd = -1; }
}

static void q12_abort_child(pid_t pid) {
  if (pid > 1) {
    kill(-pid, SIGKILL);
    kill(pid, SIGKILL);
    while (waitpid(pid, NULL, 0) < 0 && errno == EINTR) {}
  }
}

static char **q12_vector(char *first, char *blob, size_t length, int count) {
  int extra = first == NULL ? 0 : 1;
  char **result = calloc((size_t)count + (size_t)extra + 1, sizeof(char *));
  if (result == NULL) return NULL;
  if (first != NULL) result[0] = first;
  size_t offset = 0;
  for (int index = 0; index < count; index++) {
    if (offset >= length) { free(result); return NULL; }
    result[index + extra] = blob + offset;
    size_t item = strnlen(blob + offset, length - offset);
    if (item == length - offset) { free(result); return NULL; }
    offset += item + 1;
  }
  if (offset != length) { free(result); return NULL; }
  return result;
}

static int q12_hash_matches(CFDictionaryRef info, const uint8_t *expected, int count) {
  CFArrayRef hashes = CFDictionaryGetValue(info, kSecCodeInfoCdHashes);
  if (hashes == NULL || CFGetTypeID(hashes) != CFArrayGetTypeID()) return 0;
  CFIndex total = CFArrayGetCount(hashes);
  if (total < 1 || total > 64) return 0;
  for (CFIndex index = 0; index < total; index++) {
    CFDataRef value = (CFDataRef)CFArrayGetValueAtIndex(hashes, index);
    if (value == NULL || CFGetTypeID(value) != CFDataGetTypeID() ||
        CFDataGetLength(value) != 20) continue;
    for (int expected_index = 0; expected_index < count; expected_index++) {
      if (memcmp(CFDataGetBytePtr(value), expected + expected_index * 20, 20) == 0) return 1;
    }
  }
  return 0;
}

static int q12_mapped_vnode_matches(pid_t pid, uint64_t device, uint64_t inode,
                                    uint64_t links) {
  uint64_t address = 0;
  for (int index = 0; index < 4096; index++) {
    struct proc_regionwithpathinfo region;
    memset(&region, 0, sizeof(region));
    if (proc_pidinfo(pid, PROC_PIDREGIONPATHINFO, address, &region, sizeof(region)) !=
        (int)sizeof(region)) break;
    struct vinfo_stat *stat_value = &region.prp_vip.vip_vi.vi_stat;
    if ((uint64_t)stat_value->vst_dev == device && (uint64_t)stat_value->vst_ino == inode &&
        (uint64_t)stat_value->vst_nlink == links) return 1;
    uint64_t start = region.prp_prinfo.pri_address;
    uint64_t length = region.prp_prinfo.pri_size;
    if (length == 0 || start > UINT64_MAX - length || start + length <= address) break;
    address = start + length;
  }
  return 0;
}

static int q12_validate_child(pid_t pid, const char *command, const uint8_t *expected,
                              int hash_count, uint64_t device, uint64_t inode, uint64_t links) {
  CFNumberRef pid_number = CFNumberCreate(NULL, kCFNumberIntType, &pid);
  if (pid_number == NULL) return 101;
  const void *keys[] = { kSecGuestAttributePid };
  const void *values[] = { pid_number };
  CFDictionaryRef attributes = CFDictionaryCreate(NULL, keys, values, 1,
    &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
  SecCodeRef code = NULL;
  OSStatus status = attributes == NULL ? errSecAllocate :
    SecCodeCopyGuestWithAttributes(NULL, attributes, kSecCSDefaultFlags, &code);
  if (attributes != NULL) CFRelease(attributes);
  CFRelease(pid_number);
  if (status != errSecSuccess || code == NULL) return 102;
  status = SecCodeCheckValidity(code, kSecCSDefaultFlags, NULL);
  if (status != errSecSuccess) { CFRelease(code); return 103; }
  CFDictionaryRef info = NULL;
  status = SecCodeCopySigningInformation(code,
    kSecCSSigningInformation | kSecCSDynamicInformation, &info);
  if (status != errSecSuccess || info == NULL) { CFRelease(code); return 104; }
  int matched = q12_hash_matches(info, expected, hash_count);
  if (info != NULL) CFRelease(info);
  CFRelease(code);
  if (!matched) return 105;

  char process_path[PROC_PIDPATHINFO_MAXSIZE];
  char expected_path[PATH_MAX];
  char actual_path[PATH_MAX];
  struct stat stat_value;
  if (proc_pidpath(pid, process_path, sizeof(process_path)) <= 0 ||
      realpath(command, expected_path) == NULL || realpath(process_path, actual_path) == NULL ||
      strcmp(expected_path, actual_path) != 0 || lstat(command, &stat_value) != 0 ||
      !S_ISREG(stat_value.st_mode) || S_ISLNK(stat_value.st_mode) ||
      (uint64_t)stat_value.st_dev != device || (uint64_t)stat_value.st_ino != inode ||
      (uint64_t)stat_value.st_nlink != links ||
      !q12_mapped_vnode_matches(pid, device, inode, links)) return 106;
  return 0;
}

static int q12_secure_spawn(const char *command, char *args_blob, size_t args_length,
                            int args_count, char *env_blob, size_t env_length, int env_count,
                            const uint8_t *hashes, int hash_count, uint64_t device,
                            uint64_t inode, uint64_t links, q12_spawn_result *output) {
  if (command == NULL || output == NULL || hash_count < 1 || hash_count > 7) return EINVAL;
  output->pid = output->stdin_fd = output->stdout_fd = output->stderr_fd = -1;
  char **argv = q12_vector((char *)command, args_blob, args_length, args_count);
  char **envp = q12_vector(NULL, env_blob, env_length, env_count);
  if (argv == NULL || envp == NULL) { free(argv); free(envp); return ENOMEM; }
  int input[2] = {-1, -1}, stdout_pipe[2] = {-1, -1}, stderr_pipe[2] = {-1, -1};
  if (pipe(input) || pipe(stdout_pipe) || pipe(stderr_pipe)) goto fail;
  for (int index = 0; index < 2; index++) {
    fcntl(input[index], F_SETFD, FD_CLOEXEC);
    fcntl(stdout_pipe[index], F_SETFD, FD_CLOEXEC);
    fcntl(stderr_pipe[index], F_SETFD, FD_CLOEXEC);
  }
  posix_spawn_file_actions_t actions;
  posix_spawnattr_t attributes;
  if (posix_spawn_file_actions_init(&actions) != 0) goto fail;
  int actions_ready = 1, attributes_ready = 0;
  if (posix_spawnattr_init(&attributes) != 0) goto spawn_fail;
  attributes_ready = 1;
  short flags = POSIX_SPAWN_START_SUSPENDED | POSIX_SPAWN_SETPGROUP |
    POSIX_SPAWN_CLOEXEC_DEFAULT | POSIX_SPAWN_SETSIGMASK;
  sigset_t mask;
  sigemptyset(&mask);
  if (posix_spawnattr_setflags(&attributes, flags) != 0 ||
      posix_spawnattr_setpgroup(&attributes, 0) != 0 ||
      posix_spawnattr_setsigmask(&attributes, &mask) != 0) goto spawn_fail;
  if (posix_spawn_file_actions_adddup2(&actions, input[0], STDIN_FILENO) != 0 ||
      posix_spawn_file_actions_adddup2(&actions, stdout_pipe[1], STDOUT_FILENO) != 0 ||
      posix_spawn_file_actions_adddup2(&actions, stderr_pipe[1], STDERR_FILENO) != 0) goto spawn_fail;
  for (int index = 0; index < 2; index++) {
    if (posix_spawn_file_actions_addclose(&actions, input[index]) != 0 ||
        posix_spawn_file_actions_addclose(&actions, stdout_pipe[index]) != 0 ||
        posix_spawn_file_actions_addclose(&actions, stderr_pipe[index]) != 0) goto spawn_fail;
  }
  pid_t pid = -1;
  int result = posix_spawn(&pid, command, &actions, &attributes, argv, envp);
  posix_spawnattr_destroy(&attributes);
  posix_spawn_file_actions_destroy(&actions);
  free(argv); free(envp);
  if (result != 0) goto fail_no_vectors;
  q12_close(&input[0]); q12_close(&stdout_pipe[1]); q12_close(&stderr_pipe[1]);
  if (getpgid(pid) != pid) {
    q12_abort_child(pid);
    goto fail_no_vectors;
  }
  result = q12_validate_child(pid, command, hashes, hash_count, device, inode, links);
  if (result != 0 || kill(pid, SIGCONT) != 0) {
    q12_abort_child(pid);
    goto fail_no_vectors;
  }
  output->pid = pid;
  output->stdin_fd = input[1]; output->stdout_fd = stdout_pipe[0]; output->stderr_fd = stderr_pipe[0];
  return 0;

spawn_fail:
  if (attributes_ready) posix_spawnattr_destroy(&attributes);
  if (actions_ready) posix_spawn_file_actions_destroy(&actions);
fail:
  free(argv); free(envp);
fail_no_vectors:
  q12_close(&input[0]); q12_close(&input[1]); q12_close(&stdout_pipe[0]); q12_close(&stdout_pipe[1]);
  q12_close(&stderr_pipe[0]); q12_close(&stderr_pipe[1]);
  return EACCES;
}
*/
import "C"

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

func secureDesktopSpawnSupported() bool { return true }

func startSecureDesktopProcess(spec secureDesktopSpawnSpec) (*secureDesktopProcess, error) {
	args, argsCount, err := packSecureDesktopStrings(spec.arguments)
	if err != nil {
		return nil, err
	}
	environment, environmentCount, err := packSecureDesktopStrings(spec.environment)
	if err != nil {
		return nil, err
	}
	command := C.CString(spec.command)
	argsRaw := C.CBytes(args)
	environmentRaw := C.CBytes(environment)
	hashes := C.CBytes(spec.codeIdentity.bytes())
	defer C.free(unsafe.Pointer(command))
	defer C.free(argsRaw)
	defer C.free(environmentRaw)
	defer C.free(hashes)
	var result C.q12_spawn_result
	status := C.q12_secure_spawn(command, (*C.char)(argsRaw), C.size_t(len(args)), C.int(argsCount),
		(*C.char)(environmentRaw), C.size_t(len(environment)), C.int(environmentCount),
		(*C.uint8_t)(hashes), C.int(spec.codeIdentity.count), C.uint64_t(spec.fileIdentity.device),
		C.uint64_t(spec.fileIdentity.inode), C.uint64_t(spec.fileIdentity.links), &result)
	if status != 0 {
		return nil, errDesktopProviderUnavailable
	}
	process, err := os.FindProcess(int(result.pid))
	if err != nil {
		secureDesktopKillProcessGroup(int(result.pid))
		return nil, err
	}
	return &secureDesktopProcess{pid: int(result.pid), stdin: os.NewFile(uintptr(result.stdin_fd), "stdin"),
		stdout: os.NewFile(uintptr(result.stdout_fd), "stdout"), stderr: os.NewFile(uintptr(result.stderr_fd), "stderr"),
		wait: process.Wait}, nil
}

func secureDesktopKillProcessGroup(pid int) {
	if pid > 1 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func secureDesktopReapProcessGroup(pid int) error {
	if pid <= 1 {
		return nil
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		err := syscall.Kill(-pid, 0)
		if err == syscall.ESRCH {
			return nil
		}
		if err != nil && err != syscall.EPERM {
			return err
		}
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		if time.Now().After(deadline) {
			return errDesktopProviderUnavailable
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func packSecureDesktopStrings(values []string) ([]byte, int, error) {
	packed := make([]byte, 0)
	for _, value := range values {
		for index := range value {
			if value[index] == 0 {
				return nil, 0, errDesktopProviderUnavailable
			}
		}
		packed = append(packed, value...)
		packed = append(packed, 0)
	}
	return packed, len(values), nil
}

func (identity desktopCodeIdentity) bytes() []byte {
	result := make([]byte, 0, int(identity.count)*len(desktopCodeDirectoryDigest{}))
	for index := uint8(0); index < identity.count; index++ {
		result = append(result, identity.digests[index][:]...)
	}
	return result
}
