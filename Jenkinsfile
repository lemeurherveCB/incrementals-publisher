// infra.ci.jenkins.io defaults to arm64 container agents (faster in Azure) while ci.jenkins.io has the default spot amd64 used by Java builds (faster in AWS).
final String agentLabel = infra.isInfra() ? 'jnlp-linux-arm64' : 'maven-25'

pipeline {
  options {
    timeout(time: 60, unit: 'MINUTES')
    ansiColor('xterm')
    disableConcurrentBuilds(abortPrevious: true)
    buildDiscarder logRotator(artifactDaysToKeepStr: '', artifactNumToKeepStr: '', daysToKeepStr: '', numToKeepStr: '5')
  }

  agent {
    label agentLabel
  }

  environment {
    TZ = "UTC"
  }

  stages {
    stage('Check for typos') {
      steps {
        sh '''typos --format sarif > typos.sarif || true'''
      }
      post {
        always {
          recordIssues(tools: [sarif(id: 'typos', name: 'Typos', pattern: 'typos.sarif')])
        }
      }
    }

    stage('Test') {
      steps {
        sh 'go test ./...'
      }
    }

    stage('Release') {
      steps {
        buildDockerAndPublishImage('incrementals-publisher', [
          publishToPrivateAzureRegistry: true,
          targetplatforms: 'linux/arm64',
          disablePublication: !infra.isInfra(),
        ])
      }
    }
  }
}
