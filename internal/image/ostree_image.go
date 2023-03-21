package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/users"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type OSTreeImage struct {
	Base

	Platform       platform.Platform
	Workload       workload.Workload
	PartitionTable *disk.PartitionTable

	Users  []users.User
	Groups []users.Group

	Commit ostree.CommitSpec

	SysrootReadOnly bool

	Remote ostree.Remote
	OSName string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	Filename string

	Ignition         bool
	IgnitionPlatform string
	Compression      string
}

func NewOSTreeImage(commit ostree.CommitSpec) *OSTreeImage {
	return &OSTreeImage{
		Base:   NewBase("ostree-image"),
		Commit: commit,
	}
}

func ostreeCompressedImagePipelines(img *OSTreeImage, m *manifest.Manifest, buildPipeline *manifest.Build) *manifest.XZ {
	osPipeline := manifest.NewOSTreeDeployment(m, buildPipeline, img.Commit, img.OSName, img.Ignition, img.IgnitionPlatform, img.Platform)
	osPipeline.PartitionTable = img.PartitionTable
	osPipeline.Remote = img.Remote
	osPipeline.KernelOptionsAppend = img.KernelOptionsAppend
	osPipeline.Keyboard = img.Keyboard
	osPipeline.Locale = img.Locale
	osPipeline.Users = img.Users
	osPipeline.Groups = img.Groups
	osPipeline.SysrootReadOnly = img.SysrootReadOnly

	imagePipeline := manifest.NewRawOStreeImage(m, buildPipeline, img.Platform, osPipeline)

	xzPipeline := manifest.NewXZ(m, buildPipeline, imagePipeline)
	xzPipeline.Filename = img.Filename

	return xzPipeline
}

func (img *OSTreeImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOSTreeDeployment(m, buildPipeline, img.Commit, img.OSName, img.Ignition, img.IgnitionPlatform, img.Platform)
	osPipeline.PartitionTable = img.PartitionTable
	osPipeline.Remote = img.Remote
	osPipeline.KernelOptionsAppend = img.KernelOptionsAppend
	osPipeline.Keyboard = img.Keyboard
	osPipeline.Locale = img.Locale
	osPipeline.Users = img.Users
	osPipeline.Groups = img.Groups
	osPipeline.SysrootReadOnly = img.SysrootReadOnly

	imagePipeline := manifest.NewRawOStreeImage(m, buildPipeline, img.Platform, osPipeline)

	var artifact *artifact.Artifact
	var artifactPipeline manifest.Pipeline
	switch img.Platform.GetImageFormat() {
	case platform.FORMAT_RAW:
		if img.Compression == "" {
			imagePipeline.Filename = img.Filename
		}
		artifactPipeline = imagePipeline
		artifact = imagePipeline.Export()
	case platform.FORMAT_QCOW2:
		qcow2Pipeline := manifest.NewQCOW2(m, buildPipeline, imagePipeline.GetManifest(), imagePipeline.Name(), imagePipeline.Filename)
		if img.Compression == "" {
			qcow2Pipeline.Filename = img.Filename
		}
		qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()
		artifactPipeline = qcow2Pipeline
		artifact = qcow2Pipeline.Export()
	// case platform.FORMAT_VHD:
	// 	vpcPipeline := manifest.NewVPC(m, buildPipeline, imagePipeline)
	// 	if img.Compression == "" {
	// 		vpcPipeline.Filename = img.Filename
	// 	}
	// 	vpcPipeline.ForceSize = img.ForceSize
	// 	artifactPipeline = vpcPipeline
	// 	artifact = vpcPipeline.Export()
	// case platform.FORMAT_VMDK:
	// 	vmdkPipeline := manifest.NewVMDK(m, buildPipeline, imagePipeline)
	// 	if img.Compression == "" {
	// 		vmdkPipeline.Filename = img.Filename
	// 	}
	// 	artifactPipeline = vmdkPipeline
	// 	artifact = vmdkPipeline.Export()
	// case platform.FORMAT_GCE:
	// 	// NOTE(akoutsou): temporary workaround; filename required for GCP
	// 	// TODO: define internal raw filename on image type
	// 	imagePipeline.Filename = "disk.raw"
	// 	archivePipeline := manifest.NewTar(m, buildPipeline, imagePipeline, "archive")
	// 	archivePipeline.Format = osbuild.TarArchiveFormatOldgnu
	// 	archivePipeline.RootNode = osbuild.TarRootNodeOmit
	// 	// these are required to successfully import the image to GCP
	// 	archivePipeline.ACLs = common.ToPtr(false)
	// 	archivePipeline.SELinux = common.ToPtr(false)
	// 	archivePipeline.Xattrs = common.ToPtr(false)
	// 	archivePipeline.Filename = img.Filename // filename extension will determine compression
	default:
		panic("invalid image format for image kind")
	}

	switch img.Compression {
	case "xz":
		xzPipeline := manifest.NewXZ(m, buildPipeline, artifactPipeline)
		xzPipeline.Filename = img.Filename
		artifact = xzPipeline.Export()
	case "":
		// do nothing
	default:
		// panic on unknown strings
		panic(fmt.Sprintf("unsupported compression type %q", img.Compression))
	}

	return artifact, nil
}
