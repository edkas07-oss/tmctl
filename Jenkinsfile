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
        stage('Lint & Contract Validation') {
            steps {
                sh '''
                    ./scripts/validate.sh
                '''
            }
        }

        stage('Unit Testing') {
            steps {
                sh '''
                    ./scripts/test.sh
                '''
            }
        }

        stage('Build & Cross-Compilation') {
            steps {
                sh '''
                    ./scripts/build.sh
                '''
            }
        }

        stage('Archive Artifacts') {
            steps {
                archiveArtifacts artifacts: 'bin/**/*', fingerprint: true, allowEmptyArchive: false
            }
        }
    }

    post {
        success {
            echo "Pipeline tmctl build & validation completed successfully."
        }
        failure {
            echo "Pipeline tmctl build failed! Check console logs."
        }
    }
}
