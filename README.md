# CS1680 Final Project Proposal

**Team Members:** Melvin He & Dan Healey

**Project Objective:** Implement a Snowcast React application using gRPC and Protobuf

**Objectives and Scope:**
Our primary objective is to create a Snowcast application using gRPC and Protobuf within a React framework. This project aims to deepen our understanding of gRPC and Protobuf while applying them in the development of Snowcast. We seek to comprehend the differences between gRPC and conventional protocols, as well as their integration within modern frontend technologies like React. Additionally, we plan to establish a proxy to facilitate communication between the frontend and backend.

**Final Deliverables:**
1. Comprehensive documentation highlighting the distinctions between gRPC and conventional protocols, as well as their comparison with HTTP APIs.
2. A Snowcast React application demonstrating the integration of gRPC and Protobuf within a React frontend.
3. Detailed insights into our learning journey throughout the project.
4. Reach goal would be to have frontend UI with React proxy.


**Challenges and Open Questions:**
While we have a fundamental understanding of gRPC's structured message handling, we face challenges regarding the streaming of song data in the Snowcast application.

Previously, Snowcast utilized UDP for streaming. We're uncertain about the performance implications of encapsulating streaming data within structured Protobuf message types. Moreover, we seek guidance or resources for effectively playing out song data within the gRPC context.

