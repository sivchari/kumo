package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/appmesh"
	appmeshTypes "github.com/aws/aws-sdk-go-v2/service/appmesh/types"
	"github.com/spf13/cobra"
)

func newAppMeshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "appmesh",
		Short: "App Mesh commands",
	}

	cmd.AddCommand(
		// Mesh
		newAppMeshCreateMeshCmd(),
		newAppMeshDescribeMeshCmd(),
		newAppMeshListMeshesCmd(),
		newAppMeshUpdateMeshCmd(),
		newAppMeshDeleteMeshCmd(),
		// Virtual Node
		newAppMeshCreateVirtualNodeCmd(),
		newAppMeshDescribeVirtualNodeCmd(),
		newAppMeshListVirtualNodesCmd(),
		newAppMeshDeleteVirtualNodeCmd(),
		// Virtual Service
		newAppMeshCreateVirtualServiceCmd(),
		newAppMeshDescribeVirtualServiceCmd(),
		newAppMeshListVirtualServicesCmd(),
		newAppMeshDeleteVirtualServiceCmd(),
		// Virtual Router
		newAppMeshCreateVirtualRouterCmd(),
		newAppMeshDescribeVirtualRouterCmd(),
		newAppMeshListVirtualRoutersCmd(),
		newAppMeshDeleteVirtualRouterCmd(),
		// Route
		newAppMeshCreateRouteCmd(),
		newAppMeshDescribeRouteCmd(),
		newAppMeshListRoutesCmd(),
		newAppMeshDeleteRouteCmd(),
	)

	return cmd
}

func newAppmeshClient(cmd *cobra.Command) (*appmesh.Client, error) {
	cfg, err := newAWSConfig(cmd.Context())
	if err != nil {
		return nil, err
	}

	return appmesh.NewFromConfig(cfg, func(o *appmesh.Options) {
		o.BaseEndpoint = aws.String(endpointURL)
	}), nil
}

// Mesh commands

func newAppMeshCreateMeshCmd() *cobra.Command {
	var meshName, egressFilterType string

	cmd := &cobra.Command{
		Use:   "create-mesh",
		Short: "Create a service mesh",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			input := &appmesh.CreateMeshInput{
				MeshName: aws.String(meshName),
			}

			if egressFilterType != "" {
				input.Spec = &appmeshTypes.MeshSpec{
					EgressFilter: &appmeshTypes.EgressFilter{
						Type: appmeshTypes.EgressFilterType(egressFilterType),
					},
				}
			}

			out, err := client.CreateMesh(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-mesh failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&egressFilterType, "egress-filter-type", "", "Egress filter type (ALLOW_ALL, DROP_ALL)")

	return cmd
}

func newAppMeshDescribeMeshCmd() *cobra.Command {
	var meshName string

	cmd := &cobra.Command{
		Use:   "describe-mesh",
		Short: "Describe a service mesh",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.DescribeMesh(cmd.Context(), &appmesh.DescribeMeshInput{
				MeshName: aws.String(meshName),
			})
			if err != nil {
				return fmt.Errorf("describe-mesh failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")

	return cmd
}

func newAppMeshListMeshesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-meshes",
		Short: "List service meshes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.ListMeshes(cmd.Context(), &appmesh.ListMeshesInput{})
			if err != nil {
				return fmt.Errorf("list-meshes failed: %w", err)
			}

			return encodeOutput(out)
		},
	}
}

func newAppMeshUpdateMeshCmd() *cobra.Command {
	var meshName, egressFilterType string

	cmd := &cobra.Command{
		Use:   "update-mesh",
		Short: "Update a service mesh",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			input := &appmesh.UpdateMeshInput{
				MeshName: aws.String(meshName),
			}

			if egressFilterType != "" {
				input.Spec = &appmeshTypes.MeshSpec{
					EgressFilter: &appmeshTypes.EgressFilter{
						Type: appmeshTypes.EgressFilterType(egressFilterType),
					},
				}
			}

			out, err := client.UpdateMesh(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("update-mesh failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&egressFilterType, "egress-filter-type", "", "Egress filter type")

	return cmd
}

func newAppMeshDeleteMeshCmd() *cobra.Command {
	var meshName string

	cmd := &cobra.Command{
		Use:   "delete-mesh",
		Short: "Delete a service mesh",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteMesh(cmd.Context(), &appmesh.DeleteMeshInput{
				MeshName: aws.String(meshName),
			})
			if err != nil {
				return fmt.Errorf("delete-mesh failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")

	return cmd
}

// Virtual Node commands

func newAppMeshCreateVirtualNodeCmd() *cobra.Command {
	var meshName, virtualNodeName, specJSON string

	cmd := &cobra.Command{
		Use:   "create-virtual-node",
		Short: "Create a virtual node",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			input := &appmesh.CreateVirtualNodeInput{
				MeshName:        aws.String(meshName),
				VirtualNodeName: aws.String(virtualNodeName),
				Spec:            &appmeshTypes.VirtualNodeSpec{},
			}

			if specJSON != "" {
				_ = json.Unmarshal([]byte(specJSON), input.Spec)
			}

			out, err := client.CreateVirtualNode(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-virtual-node failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualNodeName, "virtual-node-name", "", "Virtual node name")
	cmd.Flags().StringVar(&specJSON, "spec", "", "Virtual node spec (JSON)")

	return cmd
}

func newAppMeshDescribeVirtualNodeCmd() *cobra.Command {
	var meshName, virtualNodeName string

	cmd := &cobra.Command{
		Use:   "describe-virtual-node",
		Short: "Describe a virtual node",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.DescribeVirtualNode(cmd.Context(), &appmesh.DescribeVirtualNodeInput{
				MeshName:        aws.String(meshName),
				VirtualNodeName: aws.String(virtualNodeName),
			})
			if err != nil {
				return fmt.Errorf("describe-virtual-node failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualNodeName, "virtual-node-name", "", "Virtual node name")

	return cmd
}

func newAppMeshListVirtualNodesCmd() *cobra.Command {
	var meshName string

	cmd := &cobra.Command{
		Use:   "list-virtual-nodes",
		Short: "List virtual nodes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.ListVirtualNodes(cmd.Context(), &appmesh.ListVirtualNodesInput{
				MeshName: aws.String(meshName),
			})
			if err != nil {
				return fmt.Errorf("list-virtual-nodes failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")

	return cmd
}

func newAppMeshDeleteVirtualNodeCmd() *cobra.Command {
	var meshName, virtualNodeName string

	cmd := &cobra.Command{
		Use:   "delete-virtual-node",
		Short: "Delete a virtual node",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteVirtualNode(cmd.Context(), &appmesh.DeleteVirtualNodeInput{
				MeshName:        aws.String(meshName),
				VirtualNodeName: aws.String(virtualNodeName),
			})
			if err != nil {
				return fmt.Errorf("delete-virtual-node failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualNodeName, "virtual-node-name", "", "Virtual node name")

	return cmd
}

// Virtual Service commands

func newAppMeshCreateVirtualServiceCmd() *cobra.Command {
	var meshName, virtualServiceName, specJSON string

	cmd := &cobra.Command{
		Use:   "create-virtual-service",
		Short: "Create a virtual service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			input := &appmesh.CreateVirtualServiceInput{
				MeshName:           aws.String(meshName),
				VirtualServiceName: aws.String(virtualServiceName),
				Spec:               &appmeshTypes.VirtualServiceSpec{},
			}

			if specJSON != "" {
				_ = json.Unmarshal([]byte(specJSON), input.Spec)
			}

			out, err := client.CreateVirtualService(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-virtual-service failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualServiceName, "virtual-service-name", "", "Virtual service name")
	cmd.Flags().StringVar(&specJSON, "spec", "", "Virtual service spec (JSON)")

	return cmd
}

func newAppMeshDescribeVirtualServiceCmd() *cobra.Command {
	var meshName, virtualServiceName string

	cmd := &cobra.Command{
		Use:   "describe-virtual-service",
		Short: "Describe a virtual service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.DescribeVirtualService(cmd.Context(), &appmesh.DescribeVirtualServiceInput{
				MeshName:           aws.String(meshName),
				VirtualServiceName: aws.String(virtualServiceName),
			})
			if err != nil {
				return fmt.Errorf("describe-virtual-service failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualServiceName, "virtual-service-name", "", "Virtual service name")

	return cmd
}

func newAppMeshListVirtualServicesCmd() *cobra.Command {
	var meshName string

	cmd := &cobra.Command{
		Use:   "list-virtual-services",
		Short: "List virtual services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.ListVirtualServices(cmd.Context(), &appmesh.ListVirtualServicesInput{
				MeshName: aws.String(meshName),
			})
			if err != nil {
				return fmt.Errorf("list-virtual-services failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")

	return cmd
}

func newAppMeshDeleteVirtualServiceCmd() *cobra.Command {
	var meshName, virtualServiceName string

	cmd := &cobra.Command{
		Use:   "delete-virtual-service",
		Short: "Delete a virtual service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteVirtualService(cmd.Context(), &appmesh.DeleteVirtualServiceInput{
				MeshName:           aws.String(meshName),
				VirtualServiceName: aws.String(virtualServiceName),
			})
			if err != nil {
				return fmt.Errorf("delete-virtual-service failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualServiceName, "virtual-service-name", "", "Virtual service name")

	return cmd
}

// Virtual Router commands

func newAppMeshCreateVirtualRouterCmd() *cobra.Command {
	var meshName, virtualRouterName, specJSON string

	cmd := &cobra.Command{
		Use:   "create-virtual-router",
		Short: "Create a virtual router",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			input := &appmesh.CreateVirtualRouterInput{
				MeshName:          aws.String(meshName),
				VirtualRouterName: aws.String(virtualRouterName),
				Spec:              &appmeshTypes.VirtualRouterSpec{},
			}

			if specJSON != "" {
				_ = json.Unmarshal([]byte(specJSON), input.Spec)
			}

			out, err := client.CreateVirtualRouter(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-virtual-router failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualRouterName, "virtual-router-name", "", "Virtual router name")
	cmd.Flags().StringVar(&specJSON, "spec", "", "Virtual router spec (JSON)")

	return cmd
}

func newAppMeshDescribeVirtualRouterCmd() *cobra.Command {
	var meshName, virtualRouterName string

	cmd := &cobra.Command{
		Use:   "describe-virtual-router",
		Short: "Describe a virtual router",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.DescribeVirtualRouter(cmd.Context(), &appmesh.DescribeVirtualRouterInput{
				MeshName:          aws.String(meshName),
				VirtualRouterName: aws.String(virtualRouterName),
			})
			if err != nil {
				return fmt.Errorf("describe-virtual-router failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualRouterName, "virtual-router-name", "", "Virtual router name")

	return cmd
}

func newAppMeshListVirtualRoutersCmd() *cobra.Command {
	var meshName string

	cmd := &cobra.Command{
		Use:   "list-virtual-routers",
		Short: "List virtual routers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.ListVirtualRouters(cmd.Context(), &appmesh.ListVirtualRoutersInput{
				MeshName: aws.String(meshName),
			})
			if err != nil {
				return fmt.Errorf("list-virtual-routers failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")

	return cmd
}

func newAppMeshDeleteVirtualRouterCmd() *cobra.Command {
	var meshName, virtualRouterName string

	cmd := &cobra.Command{
		Use:   "delete-virtual-router",
		Short: "Delete a virtual router",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteVirtualRouter(cmd.Context(), &appmesh.DeleteVirtualRouterInput{
				MeshName:          aws.String(meshName),
				VirtualRouterName: aws.String(virtualRouterName),
			})
			if err != nil {
				return fmt.Errorf("delete-virtual-router failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualRouterName, "virtual-router-name", "", "Virtual router name")

	return cmd
}

// Route commands

func newAppMeshCreateRouteCmd() *cobra.Command {
	var meshName, virtualRouterName, routeName, specJSON string

	cmd := &cobra.Command{
		Use:   cmdUseCreateRoute,
		Short: "Create a route",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			input := &appmesh.CreateRouteInput{
				MeshName:          aws.String(meshName),
				VirtualRouterName: aws.String(virtualRouterName),
				RouteName:         aws.String(routeName),
				Spec:              &appmeshTypes.RouteSpec{},
			}

			if specJSON != "" {
				_ = json.Unmarshal([]byte(specJSON), input.Spec)
			}

			out, err := client.CreateRoute(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-route failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualRouterName, "virtual-router-name", "", "Virtual router name")
	cmd.Flags().StringVar(&routeName, "route-name", "", "Route name")
	cmd.Flags().StringVar(&specJSON, "spec", "", "Route spec (JSON)")

	return cmd
}

func newAppMeshDescribeRouteCmd() *cobra.Command {
	var meshName, virtualRouterName, routeName string

	cmd := &cobra.Command{
		Use:   "describe-route",
		Short: "Describe a route",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.DescribeRoute(cmd.Context(), &appmesh.DescribeRouteInput{
				MeshName:          aws.String(meshName),
				VirtualRouterName: aws.String(virtualRouterName),
				RouteName:         aws.String(routeName),
			})
			if err != nil {
				return fmt.Errorf("describe-route failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualRouterName, "virtual-router-name", "", "Virtual router name")
	cmd.Flags().StringVar(&routeName, "route-name", "", "Route name")

	return cmd
}

func newAppMeshListRoutesCmd() *cobra.Command {
	var meshName, virtualRouterName string

	cmd := &cobra.Command{
		Use:   "list-routes",
		Short: "List routes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			out, err := client.ListRoutes(cmd.Context(), &appmesh.ListRoutesInput{
				MeshName:          aws.String(meshName),
				VirtualRouterName: aws.String(virtualRouterName),
			})
			if err != nil {
				return fmt.Errorf("list-routes failed: %w", err)
			}

			return encodeOutput(out)
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualRouterName, "virtual-router-name", "", "Virtual router name")

	return cmd
}

func newAppMeshDeleteRouteCmd() *cobra.Command {
	var meshName, virtualRouterName, routeName string

	cmd := &cobra.Command{
		Use:   "delete-route",
		Short: "Delete a route",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAppmeshClient(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteRoute(cmd.Context(), &appmesh.DeleteRouteInput{
				MeshName:          aws.String(meshName),
				VirtualRouterName: aws.String(virtualRouterName),
				RouteName:         aws.String(routeName),
			})
			if err != nil {
				return fmt.Errorf("delete-route failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&meshName, "mesh-name", "", "Mesh name")
	cmd.Flags().StringVar(&virtualRouterName, "virtual-router-name", "", "Virtual router name")
	cmd.Flags().StringVar(&routeName, "route-name", "", "Route name")

	return cmd
}

func encodeOutput(v any) error {
	if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
		return fmt.Errorf("failed to encode output: %w", err)
	}

	return nil
}
