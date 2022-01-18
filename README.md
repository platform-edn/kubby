# kubby
a library for running isolated Kubernetes clusters

# Example

    var (
        clusterName        = "test-kind-cluster"
        kubeconfigFilePath = filepath.Join(getMageDir(), "local", "kubeconfig.yaml")
        registryImage      = "registry:2"
        imageName = "test-image"
        jobName   = "test-job"
        jobSpec   = &batchv1.Job{
            ObjectMeta: metav1.ObjectMeta{
                Name:      jobName,
                Namespace: "default",
            },
            Spec: batchv1.JobSpec{
                Template: v1.PodTemplateSpec{
                    Spec: v1.PodSpec{
                        Containers: []v1.Container{
                            {
                                Name:  jobName,
                                Image: fmt.Sprintf("localhost:%v/%s", registryPort, imageName),
                            },
                        },
                        RestartPolicy: v1.RestartPolicyNever,
                    },
                },
                BackoffLimit: int32Ptr(0),
            },
        }
    )

    func int32Ptr(i int32) *int32 { return &i }

    func main() {
        cluster, err := kubby.NewKubeCluster(
            kubby.WithName(clusterName),
            kubby.WithControlNodes(1),
            kubby.WithWorkerNodes(2),
            kubby.WithKubeConfigPath(kubeconfigFilePath),
            kubby.ShouldStartOnCreation(true),
            kubby.WithMaxAttempts(10),
        )
        if err != nil {
            var ev *kubby.ExistingKubeClusterError
            if !errors.As(err, &ev) {
               panic(err)
            }
        }

        dir, err := os.Getwd()
	    if err != nil {
		    return ""
	    }

        err = cluster.Registry.PushImage(context.Background(), filepath.Join(dir, "docker"), imageName)
        if err != nil {
            err1 := cluster.Delete()
            if err1 != nil {
               panic(err)
            }

            panic(err)
        }

        err = cluster.RunJob(context.Background(), apiv1.NamespaceDefault, jobSpec, time.Second)
        if err != nil {
           panic(err)
        }

        err = cluster.Delete()
        if err != nil {
            panic(err)
        }

        return nil
    }
