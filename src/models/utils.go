// Copyright 2016 NDP Systèmes. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

import (
	"reflect"
	"strings"
)

var (
	// Testing is true if we are testing the framework
	Testing bool
)

// jsonizeExpr returns an expression slice with field names changed to the fields json names
// Computation is made relatively to the given Model
// e.g. [User Profile Name] -> [user_id profile_id name]
func jsonizeExpr(mi *Model, exprs []string) []string {
	if len(exprs) == 0 {
		return []string{}
	}
	var res []string
	fi := mi.fields.MustGet(exprs[0])
	res = append(res, fi.json)
	if len(exprs) > 1 {
		if fi.relatedModel != nil {
			res = append(res, jsonizeExpr(fi.relatedModel, exprs[1:])...)
		} else {
			log.Panic("Field is not a relation in model", "field", exprs[0], "model", mi.name)
		}
	}
	return res
}

// addNameSearchesToCondition recursively modifies the given condition to search
// on the name of the related records if they point to a relation field.
func addNameSearchesToCondition(mi *Model, cond *Condition) {
	for i, p := range cond.predicates {
		if p.cond != nil {
			addNameSearchesToCondition(mi, p.cond)
		}
		if len(p.exprs) == 0 {
			continue
		}
		fi := mi.getRelatedFieldInfo(strings.Join(p.exprs, ExprSep))
		if !fi.isRelationField() {
			continue
		}
		switch p.arg.(type) {
		case bool:
			cond.predicates[i].arg = int64(0)
		case string, ClientEvaluatedString:
			cond.predicates[i].exprs = addNameSearchToExprs(fi, p.exprs)
		}
	}
}

// addNameSearchToExprs modifies the given exprs to search on the name of the related record
// if it points to a relation field.
func addNameSearchToExprs(fi *Field, exprs []string) []string {
	relFI, exists := fi.relatedModel.fields.Get("name")
	if !exists {
		return exprs
	}
	exprsToAppend := []string{"name"}
	if relFI.isRelatedField() {
		exprsToAppend = strings.Split(relFI.relatedPath, ExprSep)
	}
	exprs = append(exprs, exprsToAppend...)
	return exprs
}

// jsonizePath returns a path with field names changed to the field json names
// Computation is made relatively to the given Model
// e.g. User.Profile.Name -> user_id.profile_id.name
func jsonizePath(mi *Model, path string) string {
	exprs := strings.Split(path, ExprSep)
	exprs = jsonizeExpr(mi, exprs)
	return strings.Join(exprs, ExprSep)
}

// filterOnDBFields returns the given fields slice with only stored fields.
// This function also adds the "id" field to the list if not present unless dontAddID is true
func filterOnDBFields(mi *Model, fields []string, dontAddID ...bool) []string {
	var res []string
	// Check if fields are stored
	for _, field := range fields {
		fieldExprs := jsonizeExpr(mi, strings.Split(field, ExprSep))
		fi := mi.fields.MustGet(fieldExprs[0])
		// Single field
		if len(fieldExprs) == 1 {
			if fi.isStored() {
				res = append(res, fi.json)
			}
			continue
		}

		// Related field (e.g. User.Profile.Age)
		if fi.relatedModel == nil {
			log.Panic("Field is not a relation in model", "field", fieldExprs[0], "model", mi.name)
		}
		subFieldName := strings.Join(fieldExprs[1:], ExprSep)
		subFieldRes := filterOnDBFields(fi.relatedModel, []string{subFieldName}, dontAddID...)
		if len(subFieldRes) == 0 {
			// Our last expr is not stored after all, we don't add anything
			continue
		}

		if !fi.isStored() {
			// We re-add our first expr as it has been removed above (not stored)
			res = append(res, fi.json)
		}
		for _, sfr := range subFieldRes {
			resExprs := []string{fi.json}
			resExprs = append(resExprs, sfr)
			res = append(res, strings.Join(resExprs, ExprSep))
		}
	}
	if len(dontAddID) == 0 || !dontAddID[0] {
		res = addIDIfNotPresent(res)
	}
	return res
}

// filterMapOnStoredFields returns a new FieldMap from fMap
// with only fields keys stored directly in this model.
//
// This function also converts all keys to fields JSON names.
func filterMapOnStoredFields(mi *Model, fMap FieldMap) FieldMap {
	newFMap := make(FieldMap)
	for field, value := range fMap {
		if fi, ok := mi.fields.Get(field); ok && fi.isStored() {
			newFMap[fi.json] = value
		}
	}
	return newFMap
}

// addIDIfNotPresent returns a new fields slice including ID if it
// is not already present. Otherwise returns the original slice.
func addIDIfNotPresent(fields []string) []string {
	var hadID bool
	for _, fName := range fields {
		if fName == "id" || fName == "ID" {
			hadID = true
		}
	}
	if !hadID {
		fields = append(fields, "id")
	}
	return fields
}

// convertToStringSlice converts the given FieldNamer slice into a slice of strings
func convertToStringSlice(fieldNames []FieldNamer) []string {
	res := make([]string, len(fieldNames))
	for i, v := range fieldNames {
		res[i] = string(v.FieldName())
	}
	return res
}

// ConvertToFieldNameSlice converts the given string fields slice into a slice of FieldNames
func ConvertToFieldNameSlice(fields []string) []FieldNamer {
	res := make([]FieldNamer, len(fields))
	for i, v := range fields {
		res[i] = FieldName(v)
	}
	return res
}

// convertToFieldNamerSlice converts the given FieldName fields slice into a slice of FieldNamers
func convertToFieldNamerSlice(fields []FieldName) []FieldNamer {
	res := make([]FieldNamer, len(fields))
	for i, v := range fields {
		res[i] = v
	}
	return res
}

// getGroupCondition returns the condition to retrieve the individual aggregated rows in vals
// knowing that they were grouped by groups and that we had the given initial condition
func getGroupCondition(groups []string, vals map[string]interface{}, initialCondition *Condition) *Condition {
	res := initialCondition
	for _, group := range groups {
		res = res.And().Field(group).Equals(vals[group])
	}
	return res
}

// substituteKeys returns a new map with its keys substituted following substMap after changing sqlSep into ExprSep.
// vals keys that are not found in substMap are not returned
func substituteKeys(vals map[string]interface{}, substMap map[string]string) map[string]interface{} {
	res := make(FieldMap)
	for f, v := range vals {
		k := strings.Replace(f, sqlSep, ExprSep, -1)
		sk, ok := substMap[k]
		if !ok {
			continue
		}
		res[sk] = v
	}
	return res
}

// serializePredicates returns a list that mimics Odoo domains from the given
// condition predicates.
func serializePredicates(predicates []predicate) []interface{} {
	var res []interface{}
	i := 0
	for i < len(predicates) {
		if predicates[i].isOr {
			subRes := []interface{}{"|"}
			subRes = appendPredicateToSerial(subRes, predicates[i])
			subRes, i = consumeAndPredicates(i+1, predicates, subRes)
			res = append(subRes, res...)
		} else {
			res, i = consumeAndPredicates(i, predicates, res)
		}
	}
	return res
}

// consumeAndPredicates appends res with all successive AND predicates
// starting from position i and returns the next position as second argument.
func consumeAndPredicates(i int, predicates []predicate, res []interface{}) ([]interface{}, int) {
	if i >= len(predicates) || predicates[i].isOr {
		return res, i
	}
	j := i
	for j < len(predicates)-1 {
		if predicates[j+1].isOr {
			break
		}
		j++
	}
	for k := i; k < j; k++ {
		res = append(res, "&")
		res = appendPredicateToSerial(res, predicates[k])
	}
	res = appendPredicateToSerial(res, predicates[j])
	return res, j + 1
}

// appendPredicateToSerial appends the given predicate to the given serialized
// predicate list and returns the result.
func appendPredicateToSerial(res []interface{}, predicate predicate) []interface{} {
	if predicate.isCond {
		res = append(res, serializePredicates(predicate.cond.predicates)...)
	} else {
		res = append(res, []interface{}{strings.Join(predicate.exprs, ExprSep), predicate.operator, predicate.arg})
	}
	return res
}

// DefaultValue returns a function that is suitable for the Default parameter of
// model fields and that simply returns value.
func DefaultValue(value interface{}) func(env Environment) interface{} {
	return func(env Environment) interface{} {
		return value
	}
}

// cartesianProductSlices returns the cartesian product of the given RecordCollection slices.
//
// This function panics if all records are not pf the same model
func cartesianProductSlices(records ...[]*RecordCollection) []*RecordCollection {
	switch len(records) {
	case 0:
		return []*RecordCollection{}
	case 1:
		return records[0]
	case 2:
		res := make([]*RecordCollection, len(records[0])*len(records[1]))
		for i, v1 := range records[0] {
			for j, v2 := range records[1] {
				res[i*len(records[1])+j] = v1.Union(v2)
			}
		}
		return res
	default:
		return cartesianProductSlices(records[0], cartesianProductSlices(records[1:]...))
	}

}

// mapToModelData maps the given FieldMap to the targetType object which is either a
// type-less ModelData or a pointer to a typed one. The given RecordCollection is used
// only to retrieve the model and an Environment.
func mapToModelData(rc *RecordCollection, fm FieldMap, targetType reflect.Type) reflect.Value {
	targetType = targetType.Elem()
	for field, val := range fm {
		fi := rc.model.Fields().MustGet(field)
		if !fi.fieldType.IsRelationType() {
			continue
		}

		var convertedValue reflect.Value
		var relRC *RecordCollection
		switch r := val.(type) {
		case RecordSet:
			relRC = r.Collection()
		case nil, *interface{}:
			relRC = newRecordCollection(rc.Env(), fi.relatedModel.name)
		case int64:
			relRC = newRecordCollection(rc.Env(), fi.relatedModel.name).withIds([]int64{r})
		case []int64:
			relRC = newRecordCollection(rc.Env(), fi.relatedModel.name).withIds(r)
		}

		if meth, ok := targetType.MethodByName(fi.name); ok {
			// We have a generated RecordSet Type with a MyField() method
			fType := meth.Type.Out(0)
			convertedValue = reflect.New(fType).Elem()
			convertedValue.FieldByName("RecordCollection").Set(reflect.ValueOf(relRC))
		} else {
			convertedValue = reflect.ValueOf(relRC)
		}
		fm[field] = convertedValue.Interface()
	}
	md := NewModelData(rc.model)
	md.FieldMap = fm
	res := reflect.New(targetType)
	res.Elem().FieldByName("ModelData").Set(reflect.ValueOf(md).Elem())
	return res
}
