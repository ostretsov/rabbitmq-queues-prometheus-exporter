package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_parseRabbitmqctlOutput(t *testing.T) {
	out := []byte(`queue1	5	4	3	2
queue2	15	3	2	1
`)
	metrics, err := parseRabbitmqctlOutput(out)
	assert.NoError(t, err)

	assert.Equal(t, "queue1", metrics[0].name)
	assert.Equal(t, 5, metrics[0].consumers)
	assert.Equal(t, 4, metrics[0].msgTotal)
	assert.Equal(t, 3, metrics[0].msgReady)
	assert.Equal(t, 2, metrics[0].msgUnack)

	assert.Equal(t, "queue2", metrics[1].name)
	assert.Equal(t, 15, metrics[1].consumers)
	assert.Equal(t, 3, metrics[1].msgTotal)
	assert.Equal(t, 2, metrics[1].msgReady)
	assert.Equal(t, 1, metrics[1].msgUnack)
}
