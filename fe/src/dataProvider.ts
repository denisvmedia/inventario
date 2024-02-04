import { DataProvider } from 'react-admin';
import axios, { AxiosInstance } from 'axios';

// eslint-disable-next-line import/no-relative-packages
import jsonapiClient from './lib/ra-jsonapi-client/src';

const options = {
  apiUrl: 'http://localhost:3333/api/v1',
};

// TODO:
// - Upload should be done as a seprate request
// - And only if create was successful
// - Upload should use a different endpoint (see in apiserver/uploads.go)

// const dataProvider = fakeRestDataProvider(data, true);
const dataProvider : DataProvider = jsonapiClient(options);

const upload = async (resource: string, fieldName: string, params: any): Promise<any> => {
  const httpClient: AxiosInstance = axios.create({
    baseURL: options.apiUrl,
    // Add more Axios configuration here (headers, etc.) as needed
  });

  const files = params.data[fieldName].map((obj: any) => obj.rawFile);

  // Append other form fields if necessary
  const response = await httpClient.postForm(`${options.apiUrl}/uploads/${resource}/${params.id}/${fieldName}`, files);

  return response;
};

const dataProviderCreate = dataProvider.create;
dataProvider.create = async (resource: string, params: any): Promise<any> => {
  if (params.data.attachments && params.data.attachments.rawFile instanceof File) {
    return upload(resource, 'attachments', params);
  }

  // Fallback for non-file data
  return dataProviderCreate(resource, params);
};

const dataProviderUpdate = dataProvider.update;
dataProvider.update = async (resource: string, params: any): Promise<any> => {
  const result = dataProviderUpdate(resource, params);

  const uploads = [];

  if (params.data.images && params.data.images.length > 0) {
    uploads.push(upload(resource, 'images', params));
  }

  if (params.data.manuals && params.data.manuals.length > 0) {
    uploads.push(upload(resource, 'manuals', params));
  }

  if (params.data.invoices && params.data.invoices.length > 0) {
    uploads.push(upload(resource, 'invoices', params));
  }

  await Promise.all(uploads);

  return result;
};

export default dataProvider;
