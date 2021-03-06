package main

import (
	"strconv"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
)

var pageSize int64 = 400

type ELB struct {
	Name  string
	Az    string
	VPCId string
	//Type  string
	Tags   []*elb.Tag
	Region string
}

func GetELBs(session *session.Session, customtags []Tag) ([]ELB, error) {
	region := session.Config.Region
	elbSvc := elb.New(session)
	input := &elb.DescribeLoadBalancersInput{
		PageSize: &pageSize,
	}
	result, err := elbSvc.DescribeLoadBalancers(input)
	if err != nil {
		return nil, err
	}
	elbs := make([]ELB, 0)
	elbnames := make([]*string, 0)
	for _, elb := range result.LoadBalancerDescriptions {
		elbnames = append(elbnames, elb.LoadBalancerName)
	}

	tagsdescriptions, err := getTagDescriptions(elbnames, elbSvc)
	if err != nil {
		return elbs, err
	}
	tagsdescriptions = filterTagDescriptions(tagsdescriptions, customtags)

	for _, elb := range result.LoadBalancerDescriptions {
		for _, td := range tagsdescriptions {
			if *elb.LoadBalancerName == *td.LoadBalancerName {
				elbs = append(elbs, ELB{
					Name:   *elb.LoadBalancerName,
					Az:     *elb.AvailabilityZones[0],
					VPCId:  *elb.VPCId,
					Tags:   td.Tags,
					Region: *region,
				})
			}
		}
	}
	return elbs, nil
}

func filterTagDescriptions(tagsdescriptions []*elb.TagDescription, customtags []Tag) []*elb.TagDescription {
	if len(customtags) == 0 {
		return tagsdescriptions
	}

	tagsdescriptions_filtered := make([]*elb.TagDescription, 0)
	var sametags int = 0
	for _, tagdesc := range tagsdescriptions {
		for _, ctag := range customtags {
			for _, tag := range tagdesc.Tags {
				if ctag.Name == *tag.Key && ctag.Value == *tag.Value {
					sametags = sametags + 1
					if sametags == len(customtags) {
						tagsdescriptions_filtered = append(tagsdescriptions_filtered, tagdesc)
						sametags = 0
					}
				}
			}
		}
	}
	return tagsdescriptions_filtered
}

func getTagDescriptions(elbnames []*string, elbSvc *elb.ELB) ([]*elb.TagDescription, error) {
	tagsdescriptions := make([]*elb.TagDescription, 0)
	inew := 0
	for i := 0; i < len(elbnames); i++ {
		inew = i + 20
		if len(elbnames) <= inew {
			inew = len(elbnames)
		}

		desctagsoutput, err := elbSvc.DescribeTags(&elb.DescribeTagsInput{
			LoadBalancerNames: elbnames[i:inew],
		})
		if err != nil {
			return nil, err
		}
		tagsdescriptions = append(tagsdescriptions, desctagsoutput.TagDescriptions...)
		i = inew
	}
	return tagsdescriptions, nil
}

func GetELBMetricProperties(elb ELB) map[string]string {
	properties := map[string]string{
		"service":          "elb",
		"name":             elb.Name,
		"az":               elb.Az,
		"vpc_id":           elb.VPCId,
		"region":           elb.Region,
		"anodot-collector": "aws",
	}

	for _, v := range elb.Tags {
		if len(*v.Key) > 50 || len(*v.Value) < 2 {
			continue
		}
		if len(properties) == 17 {
			break
		}
		properties[escape(*v.Key)] = escape(*v.Value)
	}

	for k, v := range properties {
		if len(v) > 50 || len(v) < 2 {
			delete(properties, k)
		}
	}
	return properties
}

func GetELBCloudwatchMetrics(resource *MonitoredResource, elbs []ELB) ([]MetricToFetch, error) {
	metrics := make([]MetricToFetch, 0)

	for _, mstat := range resource.Metrics {
		for _, elb := range elbs {
			m := MetricToFetch{}
			m.Dimensions = []Dimension{
				Dimension{
					Name:  "LoadBalancerName",
					Value: elb.Name,
				},
			}
			m.Resource = elb
			mstatCopy := mstat
			mstatCopy.Id = "elb" + strconv.Itoa(len(metrics))
			m.MStat = mstatCopy
			metrics = append(metrics, m)
		}
	}
	return metrics, nil
}
