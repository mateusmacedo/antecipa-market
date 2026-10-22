# syntax=docker/dockerfile:1

# Contexto de build na raiz: o go.work resolve os módulos do workspace e o kernel vem do
# proxy Go. Fora do release Docker (nx-release.yml filtra `tag:type:app,!tag:stack:go`).

FROM golang:1.26.10-alpine AS build
WORKDIR /src

COPY go.work go.work.sum ./
COPY apps/backend ./apps/backend

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
      -o /out/bff ./apps/backend/bff/cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bff /bff
COPY apps/backend/identity/contract/openapi /contracts/openapi/identity
ENV OPENAPI_IDENTITY_PATH=/contracts/openapi/identity/v1/openapi.yaml
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=4s --start-period=5s --retries=3 CMD ["/bff", "healthcheck"]
ENTRYPOINT ["/bff"]
