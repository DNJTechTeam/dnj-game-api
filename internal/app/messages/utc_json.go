package messages

import "encoding/json"

func marshalUTC(value any) ([]byte, error) { return json.Marshal(value) }
