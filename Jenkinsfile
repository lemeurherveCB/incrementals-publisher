pipeline {
  options {
    timeout(time: 60, unit: 'MINUTES')
    ansiColor('xterm')
    disableConcurrentBuilds(abortPrevious: true)
    buildDiscarder logRotator(artifactDaysToKeepStr: '', artifactNumToKeepStr: '', daysToKeepStr: '', numToKeepStr: '5')
  }

  agent {
    label 'linux-arm64-docker || arm64linux'
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
