package collectors_test

import (
	"os"

	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"

	"github.com/bosh-prometheus/bosh_exporter/deployments"
	"github.com/bosh-prometheus/bosh_exporter/filters"

	. "github.com/bosh-prometheus/bosh_exporter/collectors"
)

func init() {
	_ = log.Base().SetLevel("fatal")
}

var _ = Describe("ServiceDiscoveryCollector", func() {
	var (
		err                       error
		namespace                 string
		environment               string
		boshName                  string
		boshUUID                  string
		tmpfile                   *os.File
		serviceDiscoveryFilename  string
		azsFilter                 *filters.AZsFilter
		processesFilter           *filters.RegexpFilter
		cidrsFilter               *filters.CidrFilter
		metrics                   *ServiceDiscoveryCollectorMetrics
		serviceDiscoveryCollector *ServiceDiscoveryCollector

		lastServiceDiscoveryScrapeTimestampMetric       prometheus.Gauge
		lastServiceDiscoveryScrapeDurationSecondsMetric prometheus.Gauge
	)

	BeforeEach(func() {
		namespace = testNamespace
		environment = testEnvironment
		boshName = testBoshName
		boshUUID = testBoshUUID
		metrics = NewServiceDiscoveryCollectorMetrics(testNamespace, testEnvironment, testBoshName, testBoshUUID)
		tmpfile, err = os.CreateTemp("", "service_discovery_collector_test_")
		Expect(err).ToNot(HaveOccurred())
		serviceDiscoveryFilename = tmpfile.Name()
		azsFilter = filters.NewAZsFilter([]string{})
		cidrsFilter, err = filters.NewCidrFilter([]string{"0.0.0.0/0"})
		processesFilter, err = filters.NewRegexpFilter([]string{})

		lastServiceDiscoveryScrapeTimestampMetric = metrics.NewLastServiceDiscoveryScrapeTimestampMetric()
		lastServiceDiscoveryScrapeDurationSecondsMetric = metrics.NewLastServiceDiscoveryScrapeDurationSecondsMetric()
	})

	AfterEach(func() {
		err = os.Remove(serviceDiscoveryFilename)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		serviceDiscoveryCollector = NewServiceDiscoveryCollector(
			namespace,
			environment,
			boshName,
			boshUUID,
			serviceDiscoveryFilename,
			azsFilter,
			processesFilter,
			cidrsFilter,
		)
	})

	Describe("Describe", func() {
		var (
			descriptions chan *prometheus.Desc
		)

		BeforeEach(func() {
			descriptions = make(chan *prometheus.Desc)
		})

		JustBeforeEach(func() {
			go serviceDiscoveryCollector.Describe(descriptions)
		})

		It("returns a last_service_discovery_scrape_timestamp metric description", func() {
			Eventually(descriptions).Should(Receive(Equal(lastServiceDiscoveryScrapeTimestampMetric.Desc())))
		})

		It("returns a last_service_discovery_scrape_duration_seconds metric description", func() {
			Eventually(descriptions).Should(Receive(Equal(lastServiceDiscoveryScrapeDurationSecondsMetric.Desc())))
		})
	})

	Describe("Collect", func() {
		var (
			deployment1Name          = "fake-deployment-1-name"
			deployment2Name          = "fake-deployment-2-name"
			deployment1Release1Name  = "fake-d1-rel1"
			deployment1Release2Name  = "fake-d1-rel2"
			deployment2Release1Name  = "fake-d2-rel1"
			deploymentReleaseVersion = "fake"
			job1Name                 = "fake-job-1-name"
			job2Name                 = "fake-job-2-name"
			job1AZ                   = "fake-job-1-az"
			job2AZ                   = "fake-job-2-az"
			job1IP                   = "1.2.3.4"
			job2IP                   = "5.6.7.8"
			jobProcess1Name          = "fake-process-1-name"
			jobProcess2Name          = "fake-process-2-name"
			jobProcess3Name          = "fake-process-3-name"
			targetGroupsContent      = `[
				{"targets":["` + job1IP + `"],"labels":{"__meta_bosh_deployment":"` + deployment1Name + `","__meta_bosh_deployment_releases":"` + deployment1Release1Name + `:` + deploymentReleaseVersion + `,` + deployment1Release2Name + `:` + deploymentReleaseVersion + `","__meta_bosh_job_process_name":"` + jobProcess1Name + `","__meta_bosh_job_process_release":""}},
				{"targets":["` + job1IP + `"],"labels":{"__meta_bosh_deployment":"` + deployment1Name + `","__meta_bosh_deployment_releases":"` + deployment1Release1Name + `:` + deploymentReleaseVersion + `,` + deployment1Release2Name + `:` + deploymentReleaseVersion + `","__meta_bosh_job_process_name":"` + jobProcess2Name + `","__meta_bosh_job_process_release":""}},
				{"targets":["` + job2IP + `"],"labels":{"__meta_bosh_deployment":"` + deployment2Name + `","__meta_bosh_deployment_releases":"` + deployment2Release1Name + `:` + deploymentReleaseVersion + `","__meta_bosh_job_process_name":"` + jobProcess3Name + `","__meta_bosh_job_process_release":"` + deployment2Release1Name + `:` + deploymentReleaseVersion + `"}}
			]`

			deployment1Processes []deployments.Process
			deployment2Processes []deployments.Process
			deployment1Instances []deployments.Instance
			deployment2Instances []deployments.Instance
			deployment1Info      deployments.DeploymentInfo
			deployment2Info      deployments.DeploymentInfo
			deploymentsInfo      []deployments.DeploymentInfo

			metrics    chan prometheus.Metric
			errMetrics chan error
		)

		BeforeEach(func() {
			deployment1Processes = []deployments.Process{
				{
					Name: jobProcess1Name,
				},
				{
					Name: jobProcess2Name,
				},
			}

			deployment2Processes = []deployments.Process{
				{
					Name: jobProcess3Name,
				},
			}
			deployment1Instances = []deployments.Instance{
				{
					Name:      job1Name,
					IPs:       []string{job1IP},
					AZ:        job1AZ,
					Processes: deployment1Processes,
				},
			}

			deployment2Instances = []deployments.Instance{
				{
					Name:      job2Name,
					IPs:       []string{job2IP},
					AZ:        job2AZ,
					Processes: deployment2Processes,
				},
			}

			deployment1Info = deployments.DeploymentInfo{
				Name:      deployment1Name,
				Instances: deployment1Instances,
				Releases: []deployments.Release{
					{Name: deployment1Release1Name, Version: deploymentReleaseVersion},
					{Name: deployment1Release2Name, Version: deploymentReleaseVersion}},
			}

			deployment2Info = deployments.DeploymentInfo{
				Name:      deployment2Name,
				Instances: deployment2Instances,
				Releases: []deployments.Release{
					{
						Name:     deployment2Release1Name,
						Version:  deploymentReleaseVersion,
						JobNames: []string{jobProcess3Name},
					},
				},
			}

			deploymentsInfo = []deployments.DeploymentInfo{deployment1Info, deployment2Info}

			metrics = make(chan prometheus.Metric)
			errMetrics = make(chan error, 1)
		})

		JustBeforeEach(func() {
			go func() {
				if err := serviceDiscoveryCollector.Collect(deploymentsInfo, metrics); err != nil {
					errMetrics <- err
				}
			}()
		})

		It("writes a target groups file", func() {
			Eventually(metrics).Should(Receive())
			targetGroups, err := os.ReadFile(serviceDiscoveryFilename)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(targetGroups)).To(MatchUnorderedJSON(targetGroupsContent))
		})

		It("returns a last_service_discovery_scrape_timestamp & last_service_discovery_scrape_duration_seconds", func() {
			Eventually(metrics).Should(Receive())
			Eventually(metrics).Should(Receive())
			Consistently(metrics).ShouldNot(Receive())
			Consistently(errMetrics).ShouldNot(Receive())
		})

		Context("when there are no deployments", func() {
			BeforeEach(func() {
				deploymentsInfo = []deployments.DeploymentInfo{}
			})

			It("writes an empty target groups file", func() {
				Eventually(metrics).Should(Receive())
				targetGroups, err := os.ReadFile(serviceDiscoveryFilename)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(targetGroups)).To(Equal("[]"))
			})

			It("returns only last_service_discovery_scrape_timestamp & last_service_discovery_scrape_duration_seconds", func() {
				Eventually(metrics).Should(Receive())
				Eventually(metrics).Should(Receive())
				Consistently(metrics).ShouldNot(Receive())
				Consistently(errMetrics).ShouldNot(Receive())
			})
		})

		Context("when there are no instances", func() {
			BeforeEach(func() {
				deployment1Info.Instances = []deployments.Instance{}
				deploymentsInfo = []deployments.DeploymentInfo{deployment1Info}
			})

			It("writes an empty target groups file", func() {
				Eventually(metrics).Should(Receive())
				targetGroups, err := os.ReadFile(serviceDiscoveryFilename)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(targetGroups)).To(Equal("[]"))
			})

			It("returns only last_service_discovery_scrape_timestamp & last_service_discovery_scrape_duration_seconds", func() {
				Eventually(metrics).Should(Receive())
				Eventually(metrics).Should(Receive())
				Consistently(metrics).ShouldNot(Receive())
				Consistently(errMetrics).ShouldNot(Receive())
			})
		})

		Context("when instance has no IP", func() {
			BeforeEach(func() {
				deployment1Info.Instances[0].IPs = []string{}
				deploymentsInfo = []deployments.DeploymentInfo{deployment1Info}
			})

			It("writes an empty target groups file", func() {
				Eventually(metrics).Should(Receive())
				targetGroups, err := os.ReadFile(serviceDiscoveryFilename)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(targetGroups)).To(Equal("[]"))
			})

			It("returns only last_service_discovery_scrape_timestamp & last_service_discovery_scrape_duration_seconds", func() {
				Eventually(metrics).Should(Receive())
				Eventually(metrics).Should(Receive())
				Consistently(metrics).ShouldNot(Receive())
				Consistently(errMetrics).ShouldNot(Receive())
			})
		})

		Context("when no IP is found for an instance", func() {
			BeforeEach(func() {
				cidrsFilter, err = filters.NewCidrFilter([]string{"10.254.0.0/16"})
			})

			It("writes an empty target groups file", func() {
				Eventually(metrics).Should(Receive())
				targetGroups, err := os.ReadFile(serviceDiscoveryFilename)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(targetGroups)).To(Equal("[]"))
			})

			It("returns only last_service_discovery_scrape_timestamp & last_service_discovery_scrape_duration_seconds", func() {
				Eventually(metrics).Should(Receive())
				Eventually(metrics).Should(Receive())
				Consistently(metrics).ShouldNot(Receive())
				Consistently(errMetrics).ShouldNot(Receive())
			})
		})

		Context("when there are no processes", func() {
			BeforeEach(func() {
				deployment1Info.Instances[0].Processes = []deployments.Process{}
				deploymentsInfo = []deployments.DeploymentInfo{deployment1Info}
			})

			It("writes an empty target groups file", func() {
				Eventually(metrics).Should(Receive())
				targetGroups, err := os.ReadFile(serviceDiscoveryFilename)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(targetGroups)).To(Equal("[]"))
			})

			It("returns only last_service_discovery_scrape_timestamp & last_service_discovery_scrape_duration_seconds", func() {
				Eventually(metrics).Should(Receive())
				Eventually(metrics).Should(Receive())
				Consistently(metrics).ShouldNot(Receive())
				Consistently(errMetrics).ShouldNot(Receive())
			})
		})
	})
})
