FROM maven:3.9-eclipse-temurin-21-alpine AS builder

WORKDIR /app
COPY pom.xml ./
COPY contracts/java ./contracts/java
COPY services/identity-service/pom.xml ./services/identity-service/pom.xml
COPY services/order-service/pom.xml ./services/order-service/pom.xml
COPY services/identity-service/src ./services/identity-service/src

RUN mvn --batch-mode --no-transfer-progress clean package -pl services/identity-service -am -DskipTests

FROM eclipse-temurin:21-jre-alpine
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/services/identity-service/target/*.jar /app/app.jar

ENV SERVER_PORT=8080
ENV IDENTITY_GRPC_HOST=0.0.0.0
ENV IDENTITY_GRPC_PORT=50054
EXPOSE 8080 50054

ENTRYPOINT ["java", "-jar", "/app/app.jar"]
