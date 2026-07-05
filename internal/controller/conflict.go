package controller

import apierrors "k8s.io/apimachinery/pkg/api/errors"

func ignoreConflict(err error) error {
	if apierrors.IsConflict(err) {
		return nil
	}
	return err
}
