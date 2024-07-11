package controller

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	nodes = []node{
		{
			Enforcer:      "bpf",
			Runtime:       "cri-o",
			RuntimeSocket: "run_crio_crio.sock",
			BTF:           "yes",
			ApparmorFs:    "yes",
			Seccomp:       "no",
		},
	}
)

func TestGenerateNodeConfigHelmValues(t *testing.T) {
	nodemap := generateNodeConfigHelmValues(nodes)
	assert.NotNil(t, nodemap)
	assert.EqualValues(t, nodes[0], nodemap[0]["config"])
	log.Printf("nodemap: %+v", nodemap)
}
