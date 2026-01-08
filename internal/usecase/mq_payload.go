package usecase

import "encoding/json"

// MQPayload 包装 RabbitMQ 消息，增加类型标识
type MQPayload struct {
	Type string      `json:"type"`
	VIN  string      `json:"vin"`
	Data interface{} `json:"data"`
}

func (p MQPayload) MarshalJSON() ([]byte, error) {
	// 1. Marshal Data to get a map or basic JSON
	dataBytes, err := json.Marshal(p.Data)
	if err != nil {
		return nil, err
	}

	// 2. Try to unmarshal into a map to inject field
	var dataMap map[string]interface{}
	if err := json.Unmarshal(dataBytes, &dataMap); err == nil {
		// Injection possible
		dataMap["msgType"] = p.Type
		dataMap["vin"] = p.VIN
	} else {
		// If Data is not a struct/map (e.g. primitive), we can't inject.
		// However, for this project, Data is always a struct.
		// Fallback: Just return normal marshalling logic or wrap it differently?
		// For now, if it's not a map, we just leave it (or maybe wrap it?)
		// But let's assume it works for structs.
	}

	// 3. Create a temporary struct to marshal the final JSON to avoid infinite recursion
	type Alias MQPayload
	// We use a map for the final data if injection succeeded
	if dataMap != nil {
		return json.Marshal(&struct {
			Type string                 `json:"type"`
			VIN  string                 `json:"vin"`
			Data map[string]interface{} `json:"data"`
		}{
			Type: p.Type,
			VIN:  p.VIN,
			Data: dataMap,
		})
	}

	// Fallback to default
	return json.Marshal(Alias(p))
}
