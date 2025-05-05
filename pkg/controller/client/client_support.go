package client

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const listSuffix = "List"

// getItemGVKFromObjectList extracts the item GVK from a list ObjectList.
// This assumes the list kind ends with "List" and removes that suffix.
func getItemGVKFromObjectList(scheme *runtime.Scheme, list client.ObjectList) (schema.GroupVersionKind, error) {
	gvk, err := apiutil.GVKForObject(list, scheme)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	kind, _ := strings.CutSuffix(gvk.Kind, listSuffix)

	return schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    kind,
	}, nil
}

// unstructuredListToObjectList iterates through a slice of unstructured objects, converts
// them to the target type, and appends them to the provided client.ObjectList.
func (c *Client) unstructuredListToObjectList(
	ctx context.Context,
	items []unstructured.Unstructured,
	gvk schema.GroupVersionKind,
	list client.ObjectList,
) error {
	itemsPtr, err := meta.GetItemsPtr(list)
	if err != nil {
		return err
	}

	itemsSlice := reflect.ValueOf(itemsPtr).Elem()

	for _, u := range items {
		item, err := c.delegate.Scheme().New(gvk)
		if err != nil {
			return err
		}

		obj, ok := item.(client.Object)
		if !ok {
			return errors.New("item is not an Object")
		}

		if err := c.delegate.Scheme().Convert(&u, obj, ctx); err != nil {
			return err
		}

		// Append the converted item to the slice
		itemsSlice = reflect.Append(itemsSlice, reflect.ValueOf(item).Elem())
	}

	reflect.ValueOf(itemsPtr).Elem().Set(itemsSlice)

	return nil
}
