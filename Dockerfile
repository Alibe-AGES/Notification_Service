FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# A Lambda roda em x86_64. Sem este padrao, um build sem --build-arg gerava binario ARM
# e a Lambda falhava com "Exec format error".
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/bootstrap ./cmd/lambda

FROM alpine:3.23 AS package

RUN apk add --no-cache zip
COPY --from=build /out/bootstrap /out/bootstrap
RUN cd /out && zip -q notification-service.zip bootstrap

CMD ["cp", "/out/notification-service.zip", "/artifacts/notification-service.zip"]

FROM public.ecr.aws/lambda/provided:al2023
COPY --from=build /out/bootstrap ${LAMBDA_RUNTIME_DIR}/bootstrap
CMD ["bootstrap"]
