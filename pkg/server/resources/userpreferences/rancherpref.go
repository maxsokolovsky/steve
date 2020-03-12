package userpreferences

import (
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/store/empty"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/store/proxy"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

var (
	rancherSchema = "management.cattle.io.preference"
)

type rancherPrefStore struct {
	empty.Store
	cg proxy.ClientGetter
}

func (e *rancherPrefStore) getClient(apiOp *types.APIRequest) (dynamic.ResourceInterface, error) {
	u := getUser(apiOp).GetName()
	cmSchema := apiOp.Schemas.LookupSchema(rancherSchema)
	if cmSchema == nil {
		return nil, validation.NotFound
	}

	return e.cg.AdminClient(apiOp, cmSchema, u)
}

func (e *rancherPrefStore) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	u := getUser(apiOp)
	client, err := e.getClient(apiOp)
	if err != nil {
		return types.APIObject{}, err
	}

	pref := &UserPreference{
		Data: map[string]string{},
	}
	result := types.APIObject{
		Type:   "userpreference",
		ID:     u.GetName(),
		Object: pref,
	}

	objs, err := client.List(metav1.ListOptions{})
	if err != nil {
		return result, err
	}

	for _, obj := range objs.Items {
		pref.Data[obj.GetName()] = convert.ToString(obj.Object["value"])
	}

	return result, nil
}

func (e *rancherPrefStore) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	obj, err := e.ByID(apiOp, schema, "")
	if err != nil {
		return types.APIObjectList{}, err
	}
	return types.APIObjectList{
		Objects: []types.APIObject{
			obj,
		},
	}, nil
}

func (e *rancherPrefStore) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	client, err := e.getClient(apiOp)
	if err != nil {
		return types.APIObject{}, err
	}

	gvk := attributes.GVK(apiOp.Schemas.LookupSchema(rancherSchema))

	newValues := map[string]string{}
	for k, v := range data.Data().Map("data") {
		newValues[k] = convert.ToString(v)
	}

	prefs, err := client.List(metav1.ListOptions{})
	if err != nil {
		return types.APIObject{}, err
	}

	for _, pref := range prefs.Items {
		key := pref.GetName()
		newValue, ok := newValues[key]
		delete(newValues, key)
		if ok && newValue != pref.Object["value"] {
			pref.Object["value"] = newValue
			_, err := client.Update(&pref, metav1.UpdateOptions{})
			if err != nil {
				return types.APIObject{}, err
			}
		} else if !ok {
			err := client.Delete(key, nil)
			if err != nil {
				return types.APIObject{}, err
			}
		}
	}

	for k, v := range newValues {
		_, err = client.Create(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": gvk.GroupVersion().String(),
				"kind":       gvk.Kind,
				"metadata": map[string]interface{}{
					"name": k,
				},
				"value": v,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return types.APIObject{}, err
		}
	}

	return e.ByID(apiOp, schema, "")
}

func (e *rancherPrefStore) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	client, err := e.getClient(apiOp)
	if err != nil {
		return types.APIObject{}, err
	}

	return types.APIObject{}, client.DeleteCollection(nil, metav1.ListOptions{})
}
