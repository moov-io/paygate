achsvc
===
achsvc is a microservice enabling the easy creation of ACH files via a RESTful api. The services is designed to be deployed behind a gateway enabling access control. 

## API  
A OpenAPI definition for the service is located in the api folder. View the API specification in the swagger viewer. <link>
A mock endpoint exists at "moockbin" enabling light testing of the api. 


## Deployment 
A design goal is to be platform agnostic and integrate well with any kind of platform or infrastructure. The build directory has different deployments from a static binary to an orchestrated container in Kubernetes. 



