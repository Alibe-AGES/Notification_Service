# Alibe Notification Service

Lambda em Go que recebe notificações da fila SQS e as encaminha ao Expo Push Service.

## Contrato da mensagem SQS

Cada mensagem representa uma notificação para um único dispositivo. Essa granularidade permite que
o Lambda/SQS tente novamente apenas a entrega que falhou.

```json
{
  "to": "ExpoPushToken[xxxxxxxxxxxxxxxxxxxxxx]",
  "title": "Novo evento",
  "body": "Um evento foi criado no seu grupo.",
  "data": {
    "type": "event-created",
    "eventId": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

## Funcionamento

- O Lambda recebe as mensagens enviadas pela SQS.
- Cada mensagem é enviada ao Expo Push Service.
- A requisição ao Expo possui timeout de 8 segundos, menor que os 10 segundos da Lambda.
- JSON inválido, timeout, resposta HTTP de erro ou push ticket rejeitado volta para a fila.
- A mensagem só sai da fila quando o Expo responde com um push ticket `status: ok` e um ID.
- Esse ticket confirma que o Expo aceitou a mensagem; ele ainda não confirma a entrega no celular.

Na configuração do event source mapping da Lambda, habilite `ReportBatchItemFailures` e mantenha o
timeout de visibilidade da fila maior que o timeout da função.

## Desenvolvimento

```bash
go test ./...
go vet ./...
```

## Pacote ZIP da Lambda

O MiniStack local executa a Lambda em `x86_64`. A produção também usa `x86_64`, conforme a
estimativa aprovada. Gere o ZIP com:

```bash
docker build --target package \
  --build-arg TARGETOS=linux \
  --build-arg TARGETARCH=amd64 \
  -t alibe-notification-package .
```

O arquivo produzido pelo estágio `package` é `/out/notification-service.zip`. Em produção, ele é
enviado ao bucket de mídias já existente com uma chave versionada, por exemplo
`lambda/notification-service/<SHA>.zip`. Não é criado um segundo bucket ou repositório ECR.

O serviço não contém credenciais AWS nem credenciais do Expo. A Lambda recebe eventos por sua IAM
Role/event source mapping; as credenciais de APNs/FCM são administradas no projeto EAS/Expo.
