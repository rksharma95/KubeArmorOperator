package defaults

import (
	"crypto/sha256"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
)

const (
	// node labels
	EnforcerLabel   string = "kubearmor.io/enforcer"
	RuntimeLabel    string = "kubearmor.io/runtime"
	SocketLabel     string = "kubearmor.io/socket"
	RandLabel       string = "kubearmor.io/rand"
	OsLabel         string = "kubernetes.io/os"
	ArchLabel       string = "kubernetes.io/arch"
	BTFLabel        string = "kubearmor.io/btf"
	ApparmorFsLabel string = "kubearmor.io/apparmorfs"
	SecurityFsLabel string = "kubearmor.io/securityfs"
	SeccompLabel    string = "kubearmor.io/seccomp"

	DeleteAction string = "DELETE"
	AddAction    string = "ADD"

	SnitchName              string = "kubearmor-snitch"
	KubeArmorSnitchRoleName string = "kubearmor-snitch"
)

var (
	HostPathDirectory         = corev1.HostPathDirectory
	HostPathDirectoryOrCreate = corev1.HostPathDirectoryOrCreate
	HostPathSocket            = corev1.HostPathSocket
	HostPathFile              = corev1.HostPathFile

	Privileged bool = false
	HostPID    bool = false
)

func ShortSHA(s string) string {
	sBytes := []byte(s)

	shaFunc := sha256.New()
	shaFunc.Write(sBytes)
	res := shaFunc.Sum(nil)
	return hex.EncodeToString(res)[:5]
}
