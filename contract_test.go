package main_test

import (
	"context"
	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/config"
	"specmatic-order-bff-grpc-go/internal/com/store/order/bff/tests"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/network"
)

/*
Makes use of TestContainers to run the following:

* The Domain service (specmatic-grpc), virtualizing based on proto files.
* Kafka mock, (specmatic-kafka), to listen on product-queries.
* This BFF, system under test.
* The Test service (specmatic-grpc), to run comprehensive generative tests based on proto files.
*/
func TestContract(t *testing.T) {
	env := setUpEnv(t)

	// setUp (start domain service stub with specmatic-grpc and bff server in container)
	setUp(t, env)

	// RUN (run specmatic-grpc test in container)
	runTests(t, env)

	// TEAR DOWN
	defer tearDown(t, env)
}

func setUpEnv(t *testing.T) *tests.TestEnvironment {
	config, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	return &tests.TestEnvironment{
		Ctx:                  context.Background(),
		Config:               config,
		ExpectedMessageCount: 10,
	}
}

func setUp(t *testing.T, env *tests.TestEnvironment) {
	var err error

	// Create a sub net and store in env.
	newNetwork, err := network.New(env.Ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		require.NoError(t, newNetwork.Remove(env.Ctx))
	})
	env.DockerNetwork = newNetwork

	printHeader(t, 1, "Starting Domain Service")
	env.DomainServiceContainer, env.DomainServiceDynamicPort, err = tests.StartDomainService(env)
	if err != nil {
		t.Fatalf("could not start domain service container: %v", err)
	}

	printHeader(t, 2, "Starting Kafka Service")
	env.KafkaServiceContainer, env.KafkaServiceDynamicPort, err = tests.StartKafkaMock(env)
	if err != nil {
		t.Fatalf("could not start domain service container: %v", err)
	}

	printHeader(t, 3, "Starting BFF Service")
	env.BffServiceContainer, env.BffServiceDynamicPort, err = tests.StartBFFService(t, env)
	if err != nil {
		t.Fatalf("could not start bff service container: %v", err)
	}
}

func runTests(t *testing.T, env *tests.TestEnvironment) {
	printHeader(t, 4, "Starting tests")
	testLogs, err := tests.RunTestContainer(env)
	if err != nil {
		t.Fatalf("Could not run test container: %s", err)
	}

	// Print test outcomes
	t.Log("Test Results:")
	t.Log(testLogs)
}

func tearDown(t *testing.T, env *tests.TestEnvironment) {
	if env.BffServiceContainer != nil {
		if err := env.BffServiceContainer.Terminate(env.Ctx); err != nil {
			t.Logf("Failed to terminate BFF container: %v", err)
		}
	}
	if env.KafkaServiceContainer != nil {
		err := tests.VerifyKafkaExpectations(env)
		if err != nil {
			t.Logf("Kafka expectations were not met: %s", err)
			t.Fail()
		}
		if err := env.KafkaServiceContainer.Terminate(env.Ctx); err != nil {
			t.Logf("Failed to terminate Kafka container: %v", err)
		}
	}
	if env.DomainServiceContainer != nil {
		if err := env.DomainServiceContainer.Terminate(env.Ctx); err != nil {
			t.Logf("Failed to terminate stub container: %v", err)
		}
	}
}

func printHeader(t *testing.T, stepNum int, title string) {
	t.Log("")
	t.Logf("======== STEP %d =========", stepNum)
	t.Log(title)
	t.Log("=========================")
	t.Log("")
}
