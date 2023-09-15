package version

import (
	"github.com/prometheus/common/version"
	"k8s.io/klog/v2"
)

func init() {
	klog.Infoln(version.Print("metrics-anomaly-detector"))
}
