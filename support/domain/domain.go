/*******************************************************************************
 * Copyright 2017 Dell Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *
 * @microservice: support-domain-go library
 * @author: Ryan Comer, Dell
 * @version: 0.5.0
 *******************************************************************************/
package support_domain

type LogEntry struct {
	//Id            string   `json:"id"`
	Level         string   `json:"logLevel"`
	Labels        []string `json:"labels"`
	OriginService string   `json:"originService"`
	Message       string   `json:"message"`
	Created       int64    `json:"created"`
}

const (
	TRACE = "TRACE"
	DEBUG = "DEBUG"
	WARN  = "WARN"
	INFO  = "INFO"
	ERROR = "ERROR"
)

func IsValidLogLevel(l string) bool {
	return l == TRACE || l == DEBUG || l == WARN || l == INFO || l == ERROR
}