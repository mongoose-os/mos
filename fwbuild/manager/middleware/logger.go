/*
 * Copyright (c) 2014-2018 Cesanta Software Limited
 * All rights reserved
 *
 * Licensed under the Apache License, Version 2.0 (the ""License"");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an ""AS IS"" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package middleware

import (
	"fmt"
	"net/http"
	"time"
)

// MakeLogger creates a logger middleware suitable for using in goji
// multiplexer.
func MakeLogger() func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		mw := func(w http.ResponseWriter, r *http.Request) {
			// Start timer
			start := time.Now()
			path := r.URL.Path
			if r.URL.RawQuery != "" {
				path += "?" + r.URL.RawQuery
			}

			clientIP := r.RemoteAddr
			if ips, ok := r.Header["X-Real-Ip"]; ok {
				if len(ips) > 0 {
					clientIP = ips[0]
				}
			}

			fmt.Printf("%v START | %s | %-7s %s\n",
				start.Format("15:04:05"), clientIP, r.Method, path,
			)

			// Process request
			inner.ServeHTTP(w, r)

			// Stop timer
			end := time.Now()
			latency := end.Sub(start)

			fmt.Printf("%v END %13v | %s | %-7s %s\n",
				end.Format("15:04:05"), latency, clientIP, r.Method, path,
			)
		}
		return MkMiddleware(mw)
	}
}
