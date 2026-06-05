package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/go-resty/resty/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
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

	apiPort, err := nat.NewPort("tcp", env.Config.KafkaService.ApiPort)
	if err != nil {
		return nil, "", fmt.Errorf("invalid kafka api port: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        specmaticEnterpriseImage,
		Cmd:          []string{"stub", "--import-path=../"},
		ExposedPorts: []string{
			env.Config.Backend.Port + "/tcp",
			env.Config.KafkaService.Port + "/tcp",
			env.Config.KafkaService.ApiPort + "/tcp",
		},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(filepath.Join(pwd, "specmatic.yaml"), "/usr/src/app/specmatic.yaml"),
		),
		Networks: []string{
			env.DockerNetwork.Name,
		},
		NetworkAliases: map[string][]string{
			env.DockerNetwork.Name: {"backend-service", "kafka-service", "specmatic-kafka"},
		},
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

	mappedApiPort, err := stubContainer.MappedPort(env.Ctx, apiPort)
	if err != nil {
		return nil, "", err
	}

	kafkaExternalPort, err := kafkaExternalPortFromLogs(stubContainer, env.Ctx)
	if err != nil {
		return nil, "", err
	}

	env.Config.Backend.Host = "backend-service"
	env.Config.KafkaService.Host = "specmatic-kafka"
	env.Config.KafkaService.Port = kafkaExternalPort
	env.KafkaAPIHost, err = stubContainer.Host(env.Ctx)
	if err != nil {
		return nil, "", err
	}

	env.KafkaDynamicAPIPort = mappedApiPort.Port()
	return stubContainer, domainServicePort.Port(), nil
}

func StartBFFService(t *testing.T, env *TestEnvironment) (testcontainers.Container, string, error) {
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
		Networks: []string{
			env.DockerNetwork.Name,
		},
		NetworkAliases: map[string][]string{
			env.DockerNetwork.Name: {"bff-service"},
		},
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		},
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

	return bffContainer, env.Config.BFFServer.Port, nil
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

func SnapshotKafkaExpectations(env *TestEnvironment) error {
	client := resty.New()

	resp, err := client.R().Post(fmt.Sprintf("http://%s:%s/_specmatic/snapshot", env.KafkaAPIHost, env.KafkaDynamicAPIPort))
	if err != nil {
		return fmt.Errorf("error capturing Kafka snapshot: %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("unexpected response capturing Kafka snapshot: %s", resp.Status())
	}

	return nil
}

func kafkaExternalPortFromLogs(container testcontainers.Container, ctx context.Context) (string, error) {
	logReader, err := container.Logs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read kafka logs: %w", err)
	}
	defer logReader.Close()

	scanner := bufio.NewScanner(logReader)
	externalPortPattern := regexp.MustCompile(`EXTERNAL://specmatic-kafka:(\d+)`)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := externalPortPattern.FindStringSubmatch(line); len(matches) == 2 {
			return matches[1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to scan kafka logs: %w", err)
	}

	return "", fmt.Errorf("could not determine kafka external port from logs")
}

func VerifyKafkaExpectations(env *TestEnvironment) error {
	client := resty.New()
	resp, err := client.R().Get(fmt.Sprintf("http://%s:%s/_specmatic/verify?channels=product-queries", env.KafkaAPIHost, env.KafkaDynamicAPIPort))
	if err != nil {
		return err
	}

	counts := map[string]int{}
	if err := json.Unmarshal(resp.Body(), &counts); err != nil {
		return fmt.Errorf("failed to parse Kafka verification response: %w", err)
	}

	actualCount, ok := counts["product-queries"]
	if !ok {
		return fmt.Errorf("Kafka verification response did not include product-queries count: %v", counts)
	}

	if actualCount != env.ExpectedMessageCount {
		return fmt.Errorf("Kafka message count mismatch for product-queries: expected %d, got %d", env.ExpectedMessageCount, actualCount)
	}

	return nil
}
