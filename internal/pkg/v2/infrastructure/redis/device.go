//
// Copyright (C) 2020 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"encoding/json"
	"fmt"

	"github.com/edgexfoundry/edgex-go/internal/pkg/common"

	"github.com/edgexfoundry/go-mod-core-contracts/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v2"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"

	"github.com/gomodule/redigo/redis"
)

const (
	DeviceCollection            = "md|dv"
	DeviceCollectionName        = DeviceCollection + DBKeySeparator + v2.Name
	DeviceCollectionLabel       = DeviceCollection + DBKeySeparator + v2.Label
	DeviceCollectionServiceName = DeviceCollection + DBKeySeparator + v2.Service + DBKeySeparator + v2.Name
	DeviceCollectionProfileName = DeviceCollection + DBKeySeparator + v2.Profile + DBKeySeparator + v2.Name
)

// deviceStoredKey return the device's stored key which combines the collection name and object id
func deviceStoredKey(id string) string {
	return CreateKey(DeviceCollection, id)
}

// deviceNameExists whether the device exists by name
func deviceNameExists(conn redis.Conn, name string) (bool, errors.EdgeX) {
	exists, err := objectNameExists(conn, DeviceCollectionName, name)
	if err != nil {
		return false, errors.NewCommonEdgeX(errors.KindDatabaseError, "device existence check by name failed", err)
	}
	return exists, nil
}

// deviceIdExists checks whether the device exists by id
func deviceIdExists(conn redis.Conn, id string) (bool, errors.EdgeX) {
	exists, err := objectIdExists(conn, deviceStoredKey(id))
	if err != nil {
		return false, errors.NewCommonEdgeX(errors.KindDatabaseError, "device existence check by id failed", err)
	}
	return exists, nil
}

// addDevice adds a new device into DB
func addDevice(conn redis.Conn, d models.Device) (models.Device, errors.EdgeX) {
	exists, edgeXerr := deviceIdExists(conn, d.Id)
	if edgeXerr != nil {
		return d, errors.NewCommonEdgeXWrapper(edgeXerr)
	} else if exists {
		return d, errors.NewCommonEdgeX(errors.KindDuplicateName, fmt.Sprintf("device id %s already exists", d.Id), edgeXerr)
	}

	exists, edgeXerr = deviceNameExists(conn, d.Name)
	if edgeXerr != nil {
		return d, errors.NewCommonEdgeXWrapper(edgeXerr)
	} else if exists {
		return d, errors.NewCommonEdgeX(errors.KindDuplicateName, fmt.Sprintf("device name %s already exists", d.Name), edgeXerr)
	}

	ts := common.MakeTimestamp()
	if d.Created == 0 {
		d.Created = ts
	}
	d.Modified = ts

	dsJSONBytes, err := json.Marshal(d)
	if err != nil {
		return d, errors.NewCommonEdgeX(errors.KindContractInvalid, "unable to JSON marshal device for Redis persistence", err)
	}

	storedKey := deviceStoredKey(d.Id)
	_ = conn.Send(MULTI)
	_ = conn.Send(SET, storedKey, dsJSONBytes)
	_ = conn.Send(ZADD, DeviceCollection, 0, storedKey)
	_ = conn.Send(HSET, DeviceCollectionName, d.Name, storedKey)
	_ = conn.Send(ZADD, CreateKey(DeviceCollectionServiceName, d.ServiceName), d.Modified, storedKey)
	_ = conn.Send(ZADD, CreateKey(DeviceCollectionProfileName, d.ProfileName), d.Modified, storedKey)
	for _, label := range d.Labels {
		_ = conn.Send(ZADD, CreateKey(DeviceCollectionLabel, label), d.Modified, storedKey)
	}
	_, err = conn.Do(EXEC)
	if err != nil {
		edgeXerr = errors.NewCommonEdgeX(errors.KindDatabaseError, "device creation failed", err)
	}

	return d, edgeXerr
}

// deviceById query device by id from DB
func deviceById(conn redis.Conn, id string) (device models.Device, edgeXerr errors.EdgeX) {
	edgeXerr = getObjectById(conn, deviceStoredKey(id), &device)
	if edgeXerr != nil {
		return device, errors.NewCommonEdgeXWrapper(edgeXerr)
	}
	return
}

// deviceByName query device by name from DB
func deviceByName(conn redis.Conn, name string) (device models.Device, edgeXerr errors.EdgeX) {
	edgeXerr = getObjectByHash(conn, DeviceCollectionName, name, &device)
	if edgeXerr != nil {
		return device, errors.NewCommonEdgeXWrapper(edgeXerr)
	}
	return
}

// deleteDeviceById deletes the device by id
func deleteDeviceById(conn redis.Conn, id string) errors.EdgeX {
	device, err := deviceById(conn, id)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}
	err = deleteDevice(conn, device)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}
	return nil
}

// deleteDeviceByName deletes the device by name
func deleteDeviceByName(conn redis.Conn, name string) errors.EdgeX {
	device, err := deviceByName(conn, name)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}
	err = deleteDevice(conn, device)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}
	return nil
}

// deleteDevice deletes a device
func deleteDevice(conn redis.Conn, device models.Device) errors.EdgeX {
	storedKey := deviceStoredKey(device.Id)
	_ = conn.Send(MULTI)
	_ = conn.Send(DEL, storedKey)
	_ = conn.Send(ZREM, DeviceCollection, storedKey)
	_ = conn.Send(HDEL, DeviceCollectionName, device.Name)
	_ = conn.Send(ZREM, CreateKey(DeviceCollectionServiceName, device.ServiceName), storedKey)
	_ = conn.Send(ZREM, CreateKey(DeviceCollectionProfileName, device.ProfileName), storedKey)
	for _, label := range device.Labels {
		_ = conn.Send(ZREM, CreateKey(DeviceCollectionLabel, label), storedKey)
	}
	_, err := conn.Do(EXEC)
	if err != nil {
		return errors.NewCommonEdgeX(errors.KindDatabaseError, "device deletion failed", err)
	}
	return nil
}

// devicesByServiceName query devices by offset, limit and name
func devicesByServiceName(conn redis.Conn, offset int, limit int, name string) (devices []models.Device, edgeXerr errors.EdgeX) {
	end := offset + limit - 1
	if limit == -1 { //-1 limit means that clients want to retrieve all remaining records after offset from DB, so specifying -1 for end
		end = limit
	}
	objects, err := getObjectsByRevRange(conn, CreateKey(DeviceCollectionServiceName, name), offset, end)
	if err != nil {
		return devices, errors.NewCommonEdgeXWrapper(err)
	}

	devices = make([]models.Device, len(objects))
	for i, in := range objects {
		s := models.Device{}
		err := json.Unmarshal(in, &s)
		if err != nil {
			return []models.Device{}, errors.NewCommonEdgeX(errors.KindDatabaseError, "device format parsing failed from the database", err)
		}
		devices[i] = s
	}
	return devices, nil
}

// devicesByLabels query devices with offset, limit and labels
func devicesByLabels(conn redis.Conn, offset int, limit int, labels []string) (devices []models.Device, edgeXerr errors.EdgeX) {
	end := offset + limit - 1
	if limit == -1 { //-1 limit means that clients want to retrieve all remaining records after offset from DB, so specifying -1 for end
		end = limit
	}
	objects, edgeXerr := getObjectsByLabelsAndSomeRange(conn, ZREVRANGE, DeviceCollection, labels, offset, end)
	if edgeXerr != nil {
		return devices, errors.NewCommonEdgeXWrapper(edgeXerr)
	}

	devices = make([]models.Device, len(objects))
	for i, in := range objects {
		dp := models.Device{}
		err := json.Unmarshal(in, &dp)
		if err != nil {
			return []models.Device{}, errors.NewCommonEdgeX(errors.KindDatabaseError, "device format parsing failed from the database", err)
		}
		devices[i] = dp
	}
	return devices, nil
}

// devicesByProfileName query devices by offset, limit and profile name
func devicesByProfileName(conn redis.Conn, offset int, limit int, profileName string) (devices []models.Device, edgeXerr errors.EdgeX) {
	end := offset + limit - 1
	if limit == -1 { //-1 limit means that clients want to retrieve all remaining records after offset from DB, so specifying -1 for end
		end = limit
	}
	objects, err := getObjectsByRevRange(conn, CreateKey(DeviceCollectionProfileName, profileName), offset, end)
	if err != nil {
		return devices, errors.NewCommonEdgeXWrapper(err)
	}

	devices = make([]models.Device, len(objects))
	for i, in := range objects {
		s := models.Device{}
		err := json.Unmarshal(in, &s)
		if err != nil {
			return []models.Device{}, errors.NewCommonEdgeX(errors.KindDatabaseError, "device format parsing failed from the database", err)
		}
		devices[i] = s
	}
	return devices, nil
}
