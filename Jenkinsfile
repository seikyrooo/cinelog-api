pipeline {
    agent any

    // Trigger Poll SCM setiap 2 menit (mengecek commit baru di GitHub otomatis)
    triggers {
        pollSCM('H/2 * * * *')
    }

    stages {
        stage('1. Build Docker Image') {
            steps {
                echo '🔨 Building Cinelog API Image...'
                sh 'docker build -t cinelog-api:local .'
            }
        }

        stage('2. Load to Minikube') {
            steps {
                echo '📦 Loading Image into Minikube containerd...'
                sh 'docker save cinelog-api:local | docker exec -i minikube ctr -n k8s.io images import -'
            }
        }

        stage('3. Deploy to Kubernetes') {
            steps {
                echo '🚀 Restarting Pods in Minikube...'
                sh 'kubectl rollout restart deployment/cinelog-api -n cinelog'
                sh 'kubectl rollout status deployment/cinelog-api -n cinelog --timeout=60s'
            }
        }
    }

    post {
        success {
            echo '🎉 Cinelog API berhasil di-deploy ke Minikube!'
        }
        failure {
            echo '❌ Deploy gagal, silakan periksa log.'
        }
    }
}
