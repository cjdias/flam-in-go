// Package file is the secret store driver of the directories of secret
// files, such as those Docker mounts at /run/secrets, or Kubernetes mounts
// for the secrets of a volume: its stores read each secret from the file
// named after its key, as a
// [github.com/cjdias/flam-in-go/pkg/flam.ISecretStore].
//
// The file of a key is the key relative to the [ConfigFieldDir] of the
// store, the # of the path#field form standing for a slash, so that the
// key "db#password" reads the file db/password, and the key "api_token"
// the file api_token. A key that is empty, absolute, or holds an empty,
// "." or ".." element is refused, and the files are opened with an
// [os.Root], so that no key, nor any symbolic link, reads a file out of the
// directory. The symbolic links staying in the directory, as Kubernetes
// makes them, are followed.
//
// The value of a secret is the content of its file, without its trailing
// newline, "\n" or "\r\n", unless [ConfigFieldTrim] is false. GetMap reads
// the regular files of the directory of the path, the directory of the
// store itself for an empty path, each by its name, but those whose name
// starts with a dot, such as the ..data link of Kubernetes, and those that
// cannot be opened, such as a dangling link.
//
// The directory is opened on each call, so that it may be created, or its
// files replaced, while the store is in use.
package file
