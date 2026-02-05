package tests

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/go-resty/resty/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tidwall/gjson"
)

var specmaticEnterpriseImage = "specmatic/enterprise"

func StartDependencies(env *TestEnvironment) (testcontainers.Container, string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}

	port, err := nat.NewPort("tcp", env.Config.Backend.Port)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port number: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        specmaticEnterpriseImage,
		Cmd:          []string{"stub", "--import-path=../"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.yaml"), "/usr/src/app/specmatic.yaml"),
		),
		NetworkMode: "host",
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Stub server is running"),
			wait.ForLog("AsyncMock has started"),
		).WithStartupTimeout(2 * time.Minute),
	}

	stubContainer, err := testcontainers.GenericContainer(env.Ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}

	domainServicePort, err := stubContainer.MappedPort(env.Ctx, port)
	if err != nil {
		return nil, "", err
	}

	return stubContainer, domainServicePort.Port(), nil
}

func StartBFFService(t *testing.T, env *TestEnvironment) (testcontainers.Container, string, error) {
	port, err := nat.NewPort("tcp", env.Config.BFFServer.Port)
	if err != nil {
		return nil, "", fmt.Errorf("invalid port number: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Name: "specmatic-order-bff-grpc-go",
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       ".",
			Dockerfile:    "Dockerfile",
			PrintBuildLog: true,
			Repo:          "specmatic-bff-service-go-grpc",
			Tag:           "latest",
			KeepImage:     true,
		},
		Env: map[string]string{
			"DOMAIN_SERVER_PORT": env.Config.Backend.Port,
			"DOMAIN_SERVER_HOST": env.Config.Backend.Host,
			"KAFKA_PORT":         env.Config.KafkaService.Port,
			"KAFKA_HOST":         env.Config.KafkaService.Host,
		},
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		},
		NetworkMode: "host",
		WaitingFor:   wait.ForLog("Starting gRPC server"),
	}

	bffContainer, err := testcontainers.GenericContainer(env.Ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            true,
	})

	if err != nil {
		return nil, "", err
	}

	bffPort, err := bffContainer.MappedPort(env.Ctx, port)
	if err != nil {
		return nil, "", err
	}

	return bffContainer, bffPort.Port(), nil
}

func RunTestContainer(env *TestEnvironment) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}

	reportsDir := filepath.Join(pwd, "./build/reports/specmatic")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create reports directory: %w", err)
	}

	bffPortInt, err := strconv.Atoi(env.Config.BFFServer.Port)
	if err != nil {
		return "", fmt.Errorf("invalid port number: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image: specmaticEnterpriseImage,
		Env: map[string]string{
			"SPECMATIC_GENERATIVE_TESTS": "true",
			"PROTOC_VERSION":             "3.21.12",
		},
		Cmd: []string{"test", fmt.Sprintf("--port=%d", bffPortInt), "--host=bff-service", "--import-path=../"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.yaml"), "/usr/src/app/specmatic.yaml"),
			testcontainers.BindMount(reportsDir, "/usr/src/app/build/reports/specmatic"),
		),
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		},
		Networks: []string{
			env.DockerNetwork.Name,
		},
		WaitingFor: wait.ForLog("Tests run:"),
	}

	testContainer, err := testcontainers.GenericContainer(env.Ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", err
	}
	defer testContainer.Terminate(env.Ctx)

	// Streaming testing logs to terminal
	logReader, err := testContainer.Logs(env.Ctx)
	if err != nil {
		return "", err
	}
	defer logReader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, logReader)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func SetKafkaExpectations(env *TestEnvironment) error {
	client := resty.New()

	body := fmt.Sprintf(`{
			"expectations": [
				{ "topic": "product-queries", "count": %d }
			]
		}`, env.ExpectedMessageCount)

	_, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(fmt.Sprintf("http://%s:%s/_specmatic/expectations", env.KafkaAPIHost, env.KafkaDynamicAPIPort))

	return err
}

func VerifyKafkaExpectations(env *TestEnvironment) error {
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		Get(fmt.Sprintf("http://%s:%s/_specmatic/expectations/verification_status", env.KafkaAPIHost, env.KafkaDynamicAPIPort))
	if err != nil {
		return err
	}

	// Print the response body for debugging
	fmt.Println("Response body:", string(resp.Body()))

	if !gjson.GetBytes(resp.Body(), "success").Bool() {
		errors := gjson.GetBytes(resp.Body(), "errors").Array()
		return fmt.Errorf("Kafka mock verification failed: %v", errors)
	}
	fmt.Println("Kafka mock expectations were met successfully.")
	return nil
}
