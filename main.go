package main

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/camunda/zeebe/clients/go/v8/pkg/entities"
	"github.com/camunda/zeebe/clients/go/v8/pkg/worker"
	"github.com/camunda/zeebe/clients/go/v8/pkg/zbc"
	"google.golang.org/grpc"
)

const (
	gatewayAddress = "0.0.0.0:26500"

	// When this is a large number (e.g. 5000), the process experiences errors.
	dataCount = 5000
)

func main() {
	client := zeebeClient()

	client.NewJobWorker().JobType("first_data_loader").Handler(firstDataLoader).Name("First").Open()
	client.NewJobWorker().JobType("second_data_processor").Handler(secondParallelMultiInstance).Name("Second").Open() //.FetchVariables("inputElement")

	client.NewJobWorker().JobType("third_data_printer").Handler(thirdDataPrinter).Name("Third").Open()

	resp, err := client.NewCreateInstanceCommand().BPMNProcessId("my_process").LatestVersion().Send(context.Background())
	must(err)

	log.Printf("started process: %d", resp.ProcessInstanceKey)

	// Sleep forever.
	select {}
}

type dataWrapper struct {
	Data []data `json:"inputCollection"`
}

type data struct {
	Name      string    `json:"name"`
	Index     int       `json:"index"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func firstDataLoader(client worker.JobClient, job entities.Job) {
	log.Println("first job handler called")

	data := generateData()

	cmd, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromObject(data)
	must(err)

	_, err = cmd.Send(context.Background())
	must(err)
}

func generateData() dataWrapper {
	dw := dataWrapper{}

	for i := 0; i < dataCount; i++ {
		data := data{
			Name:      "name_" + strconv.Itoa(i),
			Index:     i,
			ExpiresAt: time.Now().Add(time.Second * time.Duration(i)),
		}

		dw.Data = append(dw.Data, data)
	}

	return dw
}

func secondParallelMultiInstance(client worker.JobClient, job entities.Job) {
	log.Println("second job handler called")

	variableMaps := make(map[string]interface{})
	variableMaps["outputElement"] = strconv.FormatInt(job.GetKey(), 10)

	command, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).VariablesFromMap(variableMaps)
	must(err)
	response, err := command.Send(context.Background())
	must(err)
	if response == nil {
		panic("Failed to receive response")
	}

}

func thirdDataPrinter(client worker.JobClient, job entities.Job) {
	log.Println("third job handler called")

	log.Printf("%+v", job.GetVariables())

	_, err := client.NewCompleteJobCommand().JobKey(job.GetKey()).Send(context.Background())
	must(err)
}

func zeebeClient() zbc.Client {
	client, err := zbc.NewClient(&zbc.ClientConfig{
		GatewayAddress:         gatewayAddress,
		UsePlaintextConnection: true,
		DialOpts:               []grpc.DialOption{grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(8 * 1024 * 1024))},
	})
	must(err)

	return client
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
