

I already have a working fullstack monorepo project:
- frontend
- backend
- dockerized services

Your task is to transform this repository into a production-ready SRE Capstone project that fully satisfies a Production Readiness Review (PRR) and maximizes grading according to the following categories:

1. Infrastructure as Code (Terraform)
2. CI/CD automation
3. Kubernetes deployment
4. Observability & Alerting
5. SRE Operations (SLOs, autoscaling, load testing)
6. Documentation & architecture
7. Production-grade reliability practices

You must:
- analyze the existing repository structure
- preserve current application logic
- add infrastructure and SRE tooling around the application
- generate clean, maintainable, production-style code
- explain every generated file
- generate missing configs
- optimize for demo quality and grading score

========================================
GENERAL REQUIREMENTS
========================================

The project must look like a real production-ready cloud-native platform.

Use:
- Docker
- Kubernetes
- Terraform
- GitHub Actions
- Prometheus
- Grafana
- Alertmanager
- Horizontal Pod Autoscaler
- Ingress
- Helm if appropriate

All infrastructure and deployment must be reproducible from scratch.

Repository should contain:
- clean folder structure
- infrastructure directory
- k8s manifests or helm charts
- monitoring stack
- CI/CD workflows
- documentation
- architecture diagrams (markdown/mermaid acceptable)

========================================
STEP 1 — REPOSITORY ANALYSIS
========================================

First:
1. Analyze the repository structure
2. Detect:
   - frontend framework
   - backend framework
   - databases
   - docker setup
   - environment variables
   - ports
   - services

========================================
STEP 2 — TERRAFORM (IaC)
========================================

Create a complete terraform setup.

Requirements:
- modular terraform architecture
- reusable modules
- variables.tf
- outputs.tf
- backend.tf
- terraform.tfvars.example

Create folders:
terraform/
  modules/
  environments/dev
  environments/prod

Provision:
- Kubernetes cluster assumptions
- networking
- namespaces
- monitoring namespace
- ingress configuration assumptions

Add:
- remote state support
- terraform formatting
- validation support

Generate:
- README for terraform usage

========================================
STEP 3 — KUBERNETES DEPLOYMENT
========================================

Containerize and deploy the application to Kubernetes.

Create:
k8s/
  backend/
  frontend/
  ingress/
  monitoring/

Requirements:
- Deployment
- Service
- ConfigMap
- Secret
- Ingress
- HPA
- Resource requests/limits

Add:
- livenessProbe
- readinessProbe
- startupProbe

Use production-style manifests.

Frontend and backend must communicate correctly.

========================================
STEP 4 — CI/CD PIPELINE
========================================

Create a full GitHub Actions CI/CD pipeline.

Requirements:
- lint
- test
- docker build
- push image to registry
- automatic kubernetes deployment

Generate:
.github/workflows/ci-cd.yml

Pipeline must:
1. run tests
2. build docker images
3. tag images
4. push images
5. deploy automatically
6. use secrets

Add:
- rollback strategy
- caching
- parallel jobs where possible

========================================
STEP 5 — OBSERVABILITY
========================================

Add full observability stack.

Install/configure:
- Prometheus
- Grafana
- Alertmanager

Backend must expose metrics endpoint.

Implement custom metrics:
- HTTP request count
- latency
- error rate
- active requests

Create Grafana dashboards for:
1. application metrics
2. infrastructure metrics
3. Kubernetes metrics

Generate alert rules:
- high error rate
- high latency
- pod restart
- high CPU usage

========================================
STEP 6 — SRE OPERATIONS
========================================

Define:
- SLIs
- SLOs
- error budgets

Create:
docs/sre.md

Add autoscaling:
- HPA based on CPU and memory

Add load testing:
- Locust configuration


Generate scripts showing traffic spikes.

========================================
STEP 7 — DOCUMENTATION
========================================

Generate professional documentation.

Required:
README.md must include:
- architecture overview
- local setup
- deployment steps
- CI/CD explanation
- monitoring explanation
- scaling explanation
- incident response section

Add:
- mermaid architecture diagrams
- deployment flow diagrams

========================================
STEP 8 — PRODUCTION HARDENING
========================================

Improve production readiness.

Add:
- .dockerignore
- .gitignore improvements
- health endpoints
- environment validation
- secure secret handling(still use .env DO NOT REMOVE IT)
- non-root docker users
- optimized Dockerfiles
- multi-stage builds

========================================
STEP 9 — DEMO OPTIMIZATION
========================================

Prepare the repository for live presentation/demo.

Create:
demo/
  demo-script.md

Demo must show:
1. Monitoring dashboards
2. Autoscaling during load
3. Alert firing
4. Kubernetes rollout

========================================
IMPORTANT
========================================

Do not rewrite the business logic unnecessarily.

Focus on:
- infrastructure
- DevOps
- SRE practices
- production readiness
- reliability
- observability
- automation

Every generated file must include comments explaining its purpose.

Act like a real SRE team preparing for a Production Readiness Review at a large tech company.
