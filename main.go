package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Secret struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata"`
	Data       map[string]string      `yaml:"data"`
}

type Deployment struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata"`
	Spec       DeploymentSpec         `yaml:"spec"`
}

type DeploymentSpec struct {
	Selector map[string]interface{} `yaml:"selector"`
	Template PodTemplate            `yaml:"template"`
}

type PodTemplate struct {
	Metadata map[string]interface{} `yaml:"metadata"`
	Spec     PodSpec                `yaml:"spec"`
}

type PodSpec struct {
	Containers []Container `yaml:"containers"`
}

type Container struct {
	Name  string   `yaml:"name"`
	Image string   `yaml:"image"`
	Ports []Port   `yaml:"ports"`
	Env   []EnvVar `yaml:"env"`
}

type Port struct {
	ContainerPort int `yaml:"containerPort"`
}

type EnvVar struct {
	Name      string        `yaml:"name"`
	ValueFrom *ValueFromRef `yaml:"valueFrom"`
}

type ValueFromRef struct {
	SecretKeyRef SecretKeyRef `yaml:"secretKeyRef"`
}

type SecretKeyRef struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
}

func main() {
	// Directory containing YAML files
	dir := "."

	// List all .yaml files in the directory
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		log.Fatalf("Failed to list YAML files: %v", err)
	}

	var secret *Secret
	var deployments []Deployment

	for _, file := range files {
		fmt.Printf("Processing file: %s\n", file)

		// Read the YAML file
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Failed to read file %s: %v\n", file, err)
			continue
		}

		// Unmarshal the YAML data into a generic map
		var genericYaml map[string]interface{}
		err = yaml.Unmarshal(data, &genericYaml)
		if err != nil {
			fmt.Printf("Failed to parse YAML in file %s: %v\n", file, err)
			continue
		}

		// Determine if the file is a Secret or a Deployment
		apiVersion, apiVersionOk := genericYaml["apiVersion"].(string)
		kind, kindOk := genericYaml["kind"].(string)

		if !apiVersionOk || !kindOk {
			fmt.Printf("File %s does not have valid apiVersion or kind: skipping\n", file)
			continue
		}

		// Process based on kind
		switch kind {
		case "Secret":
			if apiVersion == "v1" {
				var sec Secret
				err := yaml.Unmarshal(data, &sec)
				if err != nil {
					fmt.Printf("Failed to parse Secret YAML in file %s: %v\n", file, err)
					continue
				}
				secret = &sec
				fmt.Printf("Valid Secret found in file %s\n", file)
			}

		case "Deployment":
			if apiVersion == "apps/v1" {
				var dep Deployment
				err := yaml.Unmarshal(data, &dep)
				if err != nil {
					fmt.Printf("Failed to parse Deployment YAML in file %s: %v\n", file, err)
					continue
				}
				deployments = append(deployments, dep)
				fmt.Printf("Valid Deployment found in file %s\n", file)
			}

		default:
			fmt.Printf("File %s is not a Secret or Deployment: skipping\n", file)
		}
	}

	// Process the Deployment files only if a valid Secret is found
	if secret == nil {
		fmt.Println("No valid Secret found, skipping Deployment processing")
		return
	}

	for _, deployment := range deployments {
		// Clear all existing environment variables
		for i := range deployment.Spec.Template.Spec.Containers {
			deployment.Spec.Template.Spec.Containers[i].Env = []EnvVar{}
		}

		// Create a slice to hold the new environment variables
		var newEnvVars []EnvVar

		// Add environment variables from the Secret, convert names to uppercase
		for key := range secret.Data {
			newEnvVars = append(newEnvVars, EnvVar{
				Name: strings.ToUpper(key),
				ValueFrom: &ValueFromRef{
					SecretKeyRef: SecretKeyRef{
						Name: secret.Metadata["name"].(string),
						Key:  key,
					},
				},
			})
		}

		// Sort the environment variables by Name
		sort.Slice(newEnvVars, func(i, j int) bool {
			return newEnvVars[i].Name < newEnvVars[j].Name
		})

		// Assign the sorted, uppercase environment variables to the container
		for i := range deployment.Spec.Template.Spec.Containers {
			deployment.Spec.Template.Spec.Containers[i].Env = newEnvVars
		}

		// Marshal the updated Deployment YAML
		updatedDeploymentData, err := yaml.Marshal(&deployment)
		if err != nil {
			fmt.Printf("Failed to marshal updated Deployment YAML: %v\n", err)
			continue
		}

		// Write the updated Deployment YAML to a new file
		outputFile := "deployment_updated.yaml"
		outputPath := filepath.Join(dir, outputFile)
		err = os.WriteFile(outputPath, updatedDeploymentData, 0644)
		if err != nil {
			fmt.Printf("Failed to write updated Deployment file %s: %v\n", outputPath, err)
			continue
		}

		fmt.Printf("Updated Deployment YAML saved to %s\n", outputPath)
	}
}
