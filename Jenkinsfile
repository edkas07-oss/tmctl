pipeline {
    agent {
        label 'builder-01'
    }

    options {
        timeout(time: 15, unit: 'MINUTES')
        disableConcurrentBuilds()
        ansiColor('xterm')
    }

    environment {
        GO_VERSION = '1.23.6'
        CGO_ENABLED = '0'
    }

    stages {
        stage('Quality Gate 1: Contract & Static Linting') {
            steps {
                sh './scripts/validate.sh'
            }
        }

        stage('Quality Gate 2: Comprehensive Unit Testing') {
            steps {
                sh './scripts/test.sh'
            }
        }

        stage('Quality Gate 3: Deterministic Cross-Compilation') {
            steps {
                sh './scripts/build.sh'
            }
        }

        stage('Quality Gate 4: Archive Multi-OS Artifacts') {
            steps {
                archiveArtifacts artifacts: 'bin/**/*', fingerprint: true, allowEmptyArchive: false
            }
        }
    }

    post {
        always {
            cleanWs deleteDirs: true, notFailBuild: true
        }
        success {
            echo "Pipeline tmctl build & validation completed successfully."
        }
        failure {
            echo "Pipeline tmctl build failed! Check console logs."
        }
    }
}

