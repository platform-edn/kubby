package kubby

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeResourceManager struct {
	Client kubernetes.Clientset
}

func NewKubeResourceManager(kubePath string) (*KubeResourceManager, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubePath)
	if err != nil {
		return nil, fmt.Errorf("NewKubeResourceManager: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("NewKubeResourceManager: %w", err)
	}

	manager := &KubeResourceManager{
		Client: *clientset,
	}

	return manager, nil
}

func (manager *KubeResourceManager) RunJob(ctx context.Context, namespace string, jobSpec *batchv1.Job, checkInterval time.Duration) error {
	jobclient := manager.Client.BatchV1().Jobs(namespace)
	podclient := manager.Client.CoreV1().Pods(namespace)
	errChan := make(chan error)
	defer close(errChan)
	doneChan := make(chan struct{})
	wg := &sync.WaitGroup{}

	job, err := jobclient.Create(ctx, jobSpec, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("RunJob: %w", err)
	}

	pod, err := getPod(ctx, job.Name, checkInterval, podclient)
	if err != nil {
		err = deleteJob(ctx, jobclient, podclient, job, pod)
		if err != nil {
			return fmt.Errorf("RunJob: %w", err)
		}

		return fmt.Errorf("RunJob: %w", err)
	}

	wg.Add(2)
	go printLogs(ctx, podclient, pod.Name, errChan, wg)
	go checkJob(ctx, jobclient, job.Name, checkInterval, errChan, wg)
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
	case err := <-errChan:
		return fmt.Errorf("RunJob: %w", err)
	}

	err = deleteJob(ctx, jobclient, podclient, job, pod)
	if err != nil {
		return fmt.Errorf("RunJob: %w", err)
	}

	return nil
}

func (manager *KubeResourceManager) CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) error {
	client := manager.Client.AppsV1().Deployments(namespace)

	_, err := client.Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("CreateDeployment: %w", err)
	}

	return nil
}

func (manager *KubeResourceManager) DeleteDeployment(ctx context.Context, namespace string, name string) error {
	client := manager.Client.AppsV1().Deployments(namespace)

	err := client.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("DeleteDeployment: %w", err)
	}

	return nil
}

func checkJob(ctx context.Context, client v1.JobInterface, name string, interval time.Duration, errChannel chan error, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		job, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			errChannel <- fmt.Errorf("CheckJob: %w", err)
			return
		}

		if job.Status.Succeeded > 0 && job.Status.Active == 0 {
			return
		}

		if job.Status.Failed > 0 && job.Status.Active == 0 {
			errChannel <- fmt.Errorf("CheckJob: %w", &FailedJobError{
				name: name,
			})
			return
		}

		time.Sleep(interval)
	}
}

func printLogs(ctx context.Context, client corev1.PodInterface, name string, errChannel chan error, wg *sync.WaitGroup) {
	defer wg.Done()

	//TODO: should probably not let this run indefinitely
	for {
		pod, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			errChannel <- fmt.Errorf("PrintLogs: %w", err)
			return
		}

		if pod.Status.Phase == apiv1.PodPending {
			continue
		}

		break
	}

	req := client.GetLogs(name, &apiv1.PodLogOptions{})
	logs, err := req.Stream(ctx)
	if err != nil {
		errChannel <- fmt.Errorf("PrintLogs: %w", err)
		return
	}

	defer logs.Close()

	_, err = io.Copy(os.Stdout, logs)
	if err != nil {
		errChannel <- fmt.Errorf("PrintLogs: %w", err)
		return
	}
}

func getPod(ctx context.Context, name string, interval time.Duration, client corev1.PodInterface) (*apiv1.Pod, error) {
	for i := 0; i < 3; i++ {
		pods, err := client.List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("GetPod: %w", err)
		}

		for _, pod := range pods.Items {
			if pod.GenerateName == fmt.Sprintf("%s-", name) {
				return &pod, nil
			}
		}

		time.Sleep(interval)
	}

	return nil, fmt.Errorf("GetPod: %w", &BadPodNameError{
		name: name,
	})
}

func deleteJob(ctx context.Context, jobClient v1.JobInterface, podClient corev1.PodInterface, job *batchv1.Job, pod *apiv1.Pod) error {
	err := jobClient.Delete(ctx, job.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("RunJob: %s", err)
	}

	if pod != nil {
		err = podClient.Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("RunJob: %s", err)
		}
	}

	return nil
}
