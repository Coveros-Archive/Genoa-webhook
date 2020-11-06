package pkg

import (
	"context"
	"github.com/coveros/genoa/api/v1alpha1"
	cNotifyLib "github.com/coveros/notification-library"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateNamespace(name string, client client.Client) error {
	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name}, ns)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			if err := client.Create(context.TODO(), ns); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func CreateRelease(hr *v1alpha1.Release, client client.Client) (*v1alpha1.Release, error) {
	if hr.GetNamespace() == "" {
		hr.SetNamespace("default")
	}

	hrFound := &v1alpha1.Release{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: hr.GetNamespace(),
		Name:      hr.GetName(),
	}, hrFound)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return hr, client.Create(context.TODO(), hr)
		}
		return nil, err
	}

	return hrFound, nil

}

func GetChannelIDForNotification(runtimeObjMeta metav1.ObjectMeta) string {
	channelToNotify := os.Getenv("DEFAULT_CHANNEL_ID")
	if channelID, ok := runtimeObjMeta.Annotations[SlackChannelIDAnnotation]; ok {
		channelToNotify = channelID
	}
	return channelToNotify
}

func getNotificationProvider() cNotifyLib.NotificationProvider {
	notificationProvider := cNotifyLib.Noop
	if val, ok := os.LookupEnv(EnvVarNotificationProvider); ok {
		notificationProvider = cNotifyLib.NotificationProvider(val)
	}
	return notificationProvider
}

func getNotificationProviderToken() string {
	notificationProviderToken := ""
	if val, ok := os.LookupEnv(EnvVarNotificationProviderToken); ok {
		notificationProviderToken = val
	}
	return notificationProviderToken
}

func NewNotifier() cNotifyLib.Notify {
	return cNotifyLib.NewNotificationProvider(getNotificationProvider(), getNotificationProviderToken())
}



type LogAndNotify struct {
	Logger logr.Logger
	NofityInterface cNotifyLib.Notify
}

type NotifyFields struct {
	Msg string
	Channel string
	Repo string
	File string
}

func (n *NotifyFields) WithMessage(msg string) *NotifyFields {
	n.Msg = msg
	return n
}

func (l LogAndNotify)LogAndNotify(err error, fields *NotifyFields) {
	var eventType  = cNotifyLib.Success
	if err != nil {
		l.Logger.Error(err, fields.Msg)
		eventType = cNotifyLib.Failure
	} else {
		l.Logger.Info(fields.Msg)
	}

	l.NofityInterface.SendMsg(cNotifyLib.NotifyTemplate{
		Channel:   fields.Channel,
		Title:     fields.Msg,
		EventType: eventType,
		Fields: map[string]string{
			"Component":    "Genoa-webhook",
			"Reason":       fields.Msg,
			"Git-Source":   fields.Repo,
			"Release-File": fields.File,
		}})

}