package list

import (
	"github.com/alecthomas/kingpin"
	"github.com/apex/log"
	"github.com/ooni/probe-cli/internal/cli/root"
	"github.com/ooni/probe-cli/internal/database"
	"github.com/ooni/probe-cli/internal/output"
)

func init() {
	cmd := root.Command("list", "List results")

	resultID := cmd.Arg("id", "the id of the result to list measurements for").Int64()

	cmd.Action(func(_ *kingpin.ParseContext) error {
		ctx, err := root.Init()
		if err != nil {
			log.WithError(err).Error("failed to initialize root context")
			return err
		}
		if *resultID > 0 {
			measurements, err := database.ListMeasurements(ctx.DB, *resultID)
			if err != nil {
				log.WithError(err).Error("failed to list measurements")
				return err
			}

			msmtSummary := output.MeasurementSummaryData{
				TotalCount:    0,
				AnomalyCount:  0,
				DataUsageUp:   0,
				DataUsageDown: 0,
				TotalRuntime:  0,
			}
			for _, msmt := range measurements {
				if msmtSummary.TotalRuntime == 0 {
					msmtSummary.TotalRuntime = msmt.ResultRuntime
				}
				// FIXME this logic should be adjusted for test groups that have many
				// measurements in them
				if msmtSummary.DataUsageUp == 0 {
					msmtSummary.DataUsageUp = msmt.DataUsageUp
					msmtSummary.DataUsageDown = msmt.DataUsageDown
				}
				if msmt.IsAnomaly.Bool == true {
					msmtSummary.AnomalyCount++
				}
				msmtSummary.TotalCount++
				output.MeasurementItem(msmt)
			}
			output.MeasurementSummary(msmtSummary)
		} else {
			doneResults, incompleteResults, err := database.ListResults(ctx.DB)
			if err != nil {
				log.WithError(err).Error("failed to list results")
				return err
			}

			if len(incompleteResults) > 0 {
				output.SectionTitle("Incomplete results")
			}
			for idx, result := range incompleteResults {
				output.ResultItem(output.ResultItemData{
					ID:                      result.ResultID,
					Index:                   idx,
					TotalCount:              len(incompleteResults),
					Name:                    result.TestGroupName,
					StartTime:               result.StartTime,
					NetworkName:             result.Network.NetworkName,
					Country:                 result.Network.CountryCode,
					ASN:                     result.Network.ASN,
					MeasurementCount:        0,
					MeasurementAnomalyCount: 0,
					TestKeys:                "{}", // FIXME this used to be Summary we probably need to use a list now
					Done:                    result.IsDone,
					DataUsageUp:             result.DataUsageUp,
					DataUsageDown:           result.DataUsageDown,
				})
			}

			resultSummary := output.ResultSummaryData{}
			netCount := make(map[uint]int)
			output.SectionTitle("Results")
			for idx, result := range doneResults {
				totalCount, anmlyCount, err := database.GetMeasurementCounts(ctx.DB, result.ResultID)
				if err != nil {
					log.WithError(err).Error("failed to list measurement counts")
				}
				testKeys, err := database.GetResultTestKeys(ctx.DB, result.ResultID)
				if err != nil {
					log.WithError(err).Error("failed to get testKeys")
				}
				output.ResultItem(output.ResultItemData{
					ID:                      result.ResultID,
					Index:                   idx,
					TotalCount:              len(doneResults),
					Name:                    result.TestGroupName,
					StartTime:               result.StartTime,
					NetworkName:             result.Network.NetworkName,
					Country:                 result.Network.CountryCode,
					ASN:                     result.Network.ASN,
					TestKeys:                testKeys,
					MeasurementCount:        totalCount,
					MeasurementAnomalyCount: anmlyCount,
					Done:          result.IsDone,
					DataUsageUp:   result.DataUsageUp,
					DataUsageDown: result.DataUsageDown,
				})
				resultSummary.TotalTests++
				netCount[result.Network.ASN]++
				resultSummary.TotalDataUsageUp += result.DataUsageUp
				resultSummary.TotalDataUsageDown += result.DataUsageDown
			}
			resultSummary.TotalNetworks = int64(len(netCount))

			output.ResultSummary(resultSummary)
		}

		return nil
	})
}
