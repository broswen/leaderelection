package main

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"os"
	"time"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	//load env vars
	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		log.Panic().Msg("HOSTNAME env not set")
	}
	log := log.With().Str("hostname", hostname).Logger()

	lockname := os.Getenv("LOCK_NAME")
	if lockname == "" {
		log.Panic().Msg("LOCK_NAME env not set")
	}

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		log.Panic().Msg("NAMESPACE env not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//load k8s cluster config by using env vars populates by k8s
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Panic().Err(err).Msg("couldn't get in cluster config")
	}

	//create new clientset from config
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panic().Err(err).Msg("couldn't create clientset")
	}

	//define new lock resource
	l := &resourcelock.LeaseLock{
		LeaseMeta: v1.ObjectMeta{
			Name:      lockname,
			Namespace: namespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: hostname,
		},
	}
	//closes when starts leading
	leading := make(chan struct{})

	//define leader election config with logging callbacks
	lec := leaderelection.LeaderElectionConfig{
		Lock:          l,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Info().Msg("started leading")
				close(leading)
			},
			OnStoppedLeading: func() {
				log.Info().Msg("stopped leading")
				cancel()
			},
			OnNewLeader: func(identity string) {
				log.Info().Str("identity", identity).Msg("new leader")
			},
		},
		WatchDog:        nil,
		ReleaseOnCancel: true,
		Name:            hostname,
	}

	//create new leader elector client
	leaderElector, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		log.Panic().Err(err).Msg("couldn't create leader elector")
	}

	//run leader elector in separate goroutine
	go func() {
		log.Info().Msg("running leader elector")
		leaderElector.Run(ctx)
	}()

	//wait until leading
	select {
	case <-leading:
	}

	//start doing work in intervals, break loop and restart pod if context is canceled
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			log.Info().Msg("doing work...")
		case <-ctx.Done():
			log.Info().Msg("context cancelled, shutting down")
			os.Exit(0)
		}
	}
}
