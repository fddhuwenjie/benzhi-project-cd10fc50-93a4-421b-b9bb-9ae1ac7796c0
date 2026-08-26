package domain

import (
	"sort"
	"strings"
)

const MaxComponentRevisionOperations = 100

type ComponentRevisionOperation struct {
	Operation   string           `json:"operation"`
	ComponentID string           `json:"component_id,omitempty"`
	Component   *TimberComponent `json:"component,omitempty"`
}

func (c *RemediationCase) ReviseBaselineComponents(actor string, operations []ComponentRevisionOperation) (EventDraft, error) {
	if c.Status != StatusDraft || c.BaselineFrozenAt != nil {
		return EventDraft{}, Gate("baseline_components_frozen", "勘察基线冻结后禁止校订构件")
	}
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if strings.TrimSpace(actor) == "" {
		return EventDraft{}, Invalid("actor_required", "actor_id 为必填")
	}
	if len(operations) == 0 || len(operations) > MaxComponentRevisionOperations {
		return EventDraft{}, Invalid("component_operations_count", "校订操作数必须处于 1 到 %d", MaxComponentRevisionOperations)
	}

	components := make(map[string]TimberComponent, len(c.Components)+len(operations))
	for _, component := range c.Components {
		components[component.ComponentID] = component
	}
	batchIDs := make(map[string]bool, len(operations))
	results := make([]map[string]any, 0, len(operations))
	for index, operation := range operations {
		kind := strings.TrimSpace(operation.Operation)
		var componentID string
		switch kind {
		case "add", "replace":
			if operation.Component == nil || strings.TrimSpace(operation.ComponentID) != "" {
				return EventDraft{}, Invalid("component_operation_shape", "第 %d 项 %s 操作必须仅提供 component", index, kind)
			}
			if err := validateComponent(*operation.Component); err != nil {
				return EventDraft{}, err
			}
			componentID = operation.Component.ComponentID
		case "remove":
			if operation.Component != nil || strings.TrimSpace(operation.ComponentID) == "" {
				return EventDraft{}, Invalid("component_operation_shape", "第 %d 项 remove 操作必须仅提供 component_id", index)
			}
			componentID = strings.TrimSpace(operation.ComponentID)
		default:
			return EventDraft{}, Invalid("component_operation_invalid", "第 %d 项 operation 必须为 add、replace 或 remove", index)
		}
		if batchIDs[componentID] {
			return EventDraft{}, Invalid("duplicate_component_operation", "同一批次不能重复操作构件 %s", componentID)
		}
		batchIDs[componentID] = true

		_, exists := components[componentID]
		switch kind {
		case "add":
			if exists {
				return EventDraft{}, Conflict("component_exists", "构件 %s 已存在", componentID)
			}
			components[componentID] = *operation.Component
		case "replace":
			if !exists {
				return EventDraft{}, Invalid("component_not_found", "替换引用的构件 %s 不存在", componentID)
			}
			components[componentID] = *operation.Component
		case "remove":
			if !exists {
				return EventDraft{}, Invalid("component_not_found", "移除引用的构件 %s 不存在", componentID)
			}
			delete(components, componentID)
		}
		results = append(results, map[string]any{"operation": kind, "component_id": componentID})
	}
	if len(components) == 0 {
		return EventDraft{}, Gate("components_empty", "校订后至少保留一个构件")
	}
	ids := make([]string, 0, len(components))
	for id := range components {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	revised := make([]TimberComponent, 0, len(ids))
	for _, id := range ids {
		component := components[id]
		if err := validateComponent(component); err != nil {
			return EventDraft{}, err
		}
		revised = append(revised, component)
	}
	c.Components = revised
	return EventDraft{Type: "batch_baseline_components_revised", ActorID: actor, Payload: map[string]any{"results": results, "component_count": len(revised)}}, nil
}
