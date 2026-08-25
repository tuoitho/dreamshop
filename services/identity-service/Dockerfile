FROM maven:3.9-eclipse-temurin-21-alpine AS builder

WORKDIR /app
COPY pom.xml ./
COPY contracts/java ./contracts/java
COPY services ./services

RUN mvn clean package -pl services/identity-service -am -DskipTests

FROM eclipse-temurin:21-jre-alpine
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/services/identity-service/target/*.jar /app/app.jar

ENV SERVER_PORT=8080
EXPOSE 8080 50054

ENTRYPOINT ["java", "-jar", "/app/app.jar"]
