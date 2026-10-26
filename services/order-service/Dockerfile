FROM maven:3.9-eclipse-temurin-21-alpine AS builder

WORKDIR /app
COPY pom.xml ./
COPY contracts/java ./contracts/java
COPY services/identity-service/pom.xml ./services/identity-service/pom.xml
COPY services/order-service/pom.xml ./services/order-service/pom.xml
COPY services/order-service/src ./services/order-service/src

RUN mvn --batch-mode --no-transfer-progress clean package -pl services/order-service -am -DskipTests

FROM eclipse-temurin:21-jre-alpine
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/services/order-service/target/*.jar /app/app.jar

ENV SERVER_PORT=8083
ENV MANAGEMENT_SERVER_PORT=8093
EXPOSE 8083 8093

ENTRYPOINT ["java", "-jar", "/app/app.jar"]
