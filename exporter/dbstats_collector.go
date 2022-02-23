// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package exporter

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
)

type dbstatsCollector struct {
	ctx  context.Context
	base *baseCollector

	compatibleMode bool
	topologyInfo   labelsGetter

	databaseFilter []string
}

func NewDBStatsCollector(ctx context.Context, base *baseCollector, compatible bool, topology labelsGetter, databaseRegex []string) *dbstatsCollector {
	return &dbstatsCollector{
		ctx:  ctx,
		base: base,

		compatibleMode: compatible,
		topologyInfo:   topology,

		databaseFilter: databaseRegex,
	}
}

func (d *dbstatsCollector) Describe(ch chan<- *prometheus.Desc) {
	d.base.Describe(ch, d.collect)
}

func (d *dbstatsCollector) Collect(ch chan<- prometheus.Metric) {
	d.base.Collect(ch)
}

func (d *dbstatsCollector) collect(ch chan<- prometheus.Metric) {
	if d.base == nil {
		return
	}

	log := d.base.logger
	client := d.base.client

	dbNames, err := databases(d.ctx, client, d.databaseFilter, nil)
	if err != nil {
		log.Errorf("Failed to get database names: %s", err)

		return
	}

	log.Debugf("getting stats for databases: %v", dbNames)
	for _, db := range dbNames {
		var dbStats bson.M
		cmd := bson.D{{Key: "dbStats", Value: 1}, {Key: "scale", Value: 1}}
		r := client.Database(db).RunCommand(d.ctx, cmd)
		err := r.Decode(&dbStats)
		if err != nil {
			log.Errorf("Failed to get $dbstats for database %s: %s", db, err)

			continue
		}

		log.Debugf("$dbStats metrics for %s", db)
		debugResult(log, dbStats)

		prefix := "dbstats"

		labels := d.topologyInfo.baseLabels()

		// Since all dbstats will have the same fields, we need to use a label
		// to differentiate metrics between different databases.
		labels["database"] = db

		newMetrics := makeMetrics(prefix, dbStats, labels, d.compatibleMode)
		for _, metric := range newMetrics {
			ch <- metric
		}
	}
}

var _ prometheus.Collector = (*dbstatsCollector)(nil)
