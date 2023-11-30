# Snowcast App using gRPC and Protobuf

## Project Contributors
- Dan Healey
- Melvin He

## Project Outline & Objective
The primary objective of this project is to develop a Snowcast music application utilizing gRPC and Protobuf. The aim is to deepen understanding of these technologies while implementing them in Snowcast's development. We intend to explore the disparities between gRPC and traditional protocols, examining their compatibility with the previous UDP snowcast music server. Additionally, we aim to investigate potential integration possibilities with modern frontend frameworks/technologies like React.

### Reach Goals
If time permits, we aspire to establish a proxy for enhancing communication between the frontend and backend. This would enable our gRPC snowcast implementation to seamlessly integrate with a React app. As a reach goal, we aim to empower users to select, send, and listen to music through a user-friendly React interface, departing from the terminal-based interaction.

## Tools, Languages, and Libraries
- **Primary Languages**: GoLang for snowcast music server functionalities, and potentially TypeScript/JavaScript for the React frontend.
- **Communication & Serialization**: gRPC and Protobuf for backend communication and message serialization/deserialization.
- **Frontend Framework**: Considering React for the frontend framework, integrating gRPC for backend communication and Protobuf for defining message formats. Python is also under consideration.

## Open Questions
- **Performance Concerns**: Uncertainty surrounds the performance implications of encapsulating streaming data within structured Protobuf message types, especially in contrast to Snowcast's prior use of UDP for streaming.
- **Playing Song Data**: Seeking guidance or resources on effectively playing song data within the context of gRPC.

