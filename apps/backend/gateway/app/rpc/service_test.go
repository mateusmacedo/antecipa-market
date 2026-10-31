package rpc_test

import (
	"slices"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
)

var descriptor = servicev1.File_company_gateway_service_v1_gateway_service_proto.Services().ByName("GatewayService")

func TestServiceDescCoversEveryMethodOfTheContractOnceAsUnary(t *testing.T) {
	if uncovered := kernelgrpc.Uncovered(&rpc.ServiceDesc, descriptor); len(uncovered) != 0 {
		t.Fatalf("Uncovered() = %v, want every method registered once", uncovered)
	}
	if rpc.ServiceDesc.ServiceName != "company.gateway.service.v1.GatewayService" || len(rpc.ServiceDesc.Streams) != 0 {
		t.Fatalf("ServiceDesc = %q with %d streams", rpc.ServiceDesc.ServiceName, len(rpc.ServiceDesc.Streams))
	}
}

func TestEveryCommandIsAMethodAndNoQueryIsACommand(t *testing.T) {
	methods := rpc.Methods()
	for _, command := range rpc.Commands() {
		if !slices.Contains(methods, command) {
			t.Errorf("command %s is not a method of the contract", command)
		}
	}
	for _, method := range methods {
		if slices.Contains(rpc.Commands(), method) == (len(method) > 4 && method[:4] == "Find") {
			t.Errorf("method %s is a command iff it is not a query", method)
		}
	}
}

func TestACommandWithoutKeyIsRefusedBeforeTheHandler(t *testing.T) {
	h := newHarness(t)

	err := h.invoke(withoutKey(t), "RegisterConnector", registerRequest(), &servicev1.RegisterConnectorResponse{})

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("RegisterConnector() without key = %v, want InvalidArgument", err)
	}
}
