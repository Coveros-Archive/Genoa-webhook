package gitSync

import (
	"context"
	"fmt"
	"github.com/agill17/go-scm/scm"
	utils "github.com/coveros/genoa-webhook/pkg"
	"github.com/coveros/genoa/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"reflect"
)

func (wH WebhookHandler) syncReleaseWithGithub(ownerRepo, branch, SHA, releaseFile string, scmClient *scm.Client, isRemovedFromGithub bool) {
	var readFileFrom = branch
	if isRemovedFromGithub {
		readFileFrom = SHA
	}

	wH.Logger.Info(fmt.Sprintf("Attempting to sync %v from %v into cluster", releaseFile, ownerRepo))

	scmFileContents, _, errGettingFileContents := scmClient.Contents.Find(context.TODO(), ownerRepo, releaseFile, readFileFrom)
	if errGettingFileContents != nil {
		wH.Logger.Error(errGettingFileContents, "Failed to get file contents from git")
		return
	}
	gitFileContents := string(scmFileContents.Data)

	hrFromGit := &v1alpha1.Release{}
	_, gvk, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(gitFileContents), nil, hrFromGit)
	if err != nil {
		wH.Logger.Error(err, "Could not decode release file from git, perhaps its not a release file..")
		return
	}

	if gvk.Kind != "Release" && gvk.GroupVersion() != v1alpha1.GroupVersion {
		wH.Logger.Info(fmt.Sprintf("Not a valid release %v from %v git repo", releaseFile, ownerRepo))
		return
	}

	if hrFromGit.Spec.ValuesOverride.V == nil {
		hrFromGit.Spec.ValuesOverride.V = map[string]interface{}{}
	}

	if hrFromGit.GetNamespace() == "" {
		hrFromGit.SetNamespace("default")
	}

	notificationChannel := utils.GetChannelIDForNotification(hrFromGit.ObjectMeta)
	namespacedName := fmt.Sprintf("%s/%s", hrFromGit.GetNamespace(), hrFromGit.GetName())
	ownerRepoBranch := fmt.Sprintf("%v@%v", ownerRepo, branch)
	notifyFields := utils.NotifyFields{Channel:notificationChannel, Repo:ownerRepoBranch, File:releaseFile}
	logAndNotify := utils.LogAndNotify{NofityInterface: wH.Notify,Logger: wH.Logger}

	// if GitBranchToFollowAnnotation is specified, we ONLY create/update CR's if the current source branch is the same as GitBranchToFollow
	// this way users can have same CR's exist on many branches but only apply updates from the GitBranchToFollow
	if branchToFollow, ok := hrFromGit.Annotations[utils.GitBranchToFollowAnnotation]; ok && branchToFollow != "" {
		if branchToFollow != branch {
			wH.Logger.Info(fmt.Sprintf("%v from %v, follow-git-branch '%v' does not match current branch '%v'",
				hrFromGit.GetName(), ownerRepo, branchToFollow, branch))
			return
		}
	}

	if isRemovedFromGithub {
		if err := wH.Client.Delete(context.TODO(), hrFromGit); err != nil {
			if errors.IsNotFound(err) {
				wH.Logger.Info(fmt.Sprintf("%v/%v release not found, skipping clean up..", hrFromGit.GetNamespace(), hrFromGit.GetName()))
				return
			}
			logAndNotify.LogAndNotify(err, notifyFields.WithMessage("Failed to delete Release which was removed from github"))
			return
		}
		logAndNotify.LogAndNotify(nil, notifyFields.WithMessage(fmt.Sprintf("Delete %v release from cluster initiated...", hrFromGit.GetName())))
		return
	}

	wH.Logger.Info(fmt.Sprintf("Creating %v namespace if needed..", hrFromGit.GetNamespace()))
	if errCreatingNamespace := utils.CreateNamespace(hrFromGit.GetNamespace(), wH.Client); errCreatingNamespace != nil {
		wH.Logger.Error(errCreatingNamespace, "Failed to create namespace")
		return
	}

	wH.Logger.Info(fmt.Sprintf("Creating %v/%v release", hrFromGit.GetNamespace(), hrFromGit.GetName()))
	hrFromCluster, errCreatingHR := utils.CreateRelease(hrFromGit, wH.Client)
	if errCreatingHR != nil {
		logAndNotify.LogAndNotify(errCreatingHR, notifyFields.WithMessage(fmt.Sprintf("%v failed to create release : %v", namespacedName, errCreatingHR)))
	}

	logAndNotify.LogAndNotify(nil, notifyFields.WithMessage(fmt.Sprintf("Successfully created %v release in your cluster", namespacedName)))

	specInSync := reflect.DeepEqual(hrFromCluster.Spec, hrFromGit.Spec)
	labelsInSync := reflect.DeepEqual(hrFromCluster.GetLabels(), hrFromGit.GetLabels())
	annotationsInSync := reflect.DeepEqual(hrFromCluster.GetAnnotations(), hrFromGit.GetAnnotations())
	if !specInSync || !labelsInSync || !annotationsInSync {
		hrFromCluster.SetAnnotations(hrFromGit.GetAnnotations())
		hrFromCluster.SetLabels(hrFromGit.GetLabels())
		hrFromCluster.Spec = hrFromGit.Spec
		if errUpdating := wH.Client.Update(context.TODO(), hrFromCluster); errUpdating != nil {
			notifyFields.Msg =  fmt.Sprintf("Failed to apply release from %v - %v", ownerRepo, namespacedName)
			logAndNotify.LogAndNotify(errUpdating, notifyFields.WithMessage(fmt.Sprintf("Failed to apply release from %v - %v", ownerRepo, namespacedName)))
			return
		}

		logAndNotify.LogAndNotify(nil, notifyFields.WithMessage(fmt.Sprintf("Updated release from %v - %v", ownerRepo, namespacedName)))
	}

}
