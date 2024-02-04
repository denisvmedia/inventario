import simpleRestDataProvider from 'ra-data-simple-rest';
import {
  CreateParams,
  UpdateParams,
  DataProvider,
  fetchUtils,
} from 'react-admin';

const endpoint = 'http://localhost:3333/api/v1';
const baseDataProvider = simpleRestDataProvider(endpoint);

type PostParams = {
  id: string;
  title: string;
  content: string;
  picture: {
    rawFile: File;
    src?: string;
    title?: string;
  };
};

const createPostFormData = (
  params: CreateParams<PostParams> | UpdateParams<PostParams>,
) => {
  const formData = new FormData();
  if (params.data.picture?.rawFile) {
    formData.append('file', params.data.picture.rawFile);
  }
  if (params.data.title) {
    formData.append('title', params.data.title);
  }
  if (params.data.content) {
    formData.append('content', params.data.content);
  }

  return formData;
};

const dataProvider: DataProvider = {
  ...baseDataProvider,
  create: (resource, params) => {
    if (resource === 'posts') {
      const formData = createPostFormData(params);
      return fetchUtils
        .fetchJson(`${endpoint}/${resource}`, {
          method: 'POST',
          body: formData,
        })
        .then(({ json }) => ({ data: json }));
    }

    return baseDataProvider.create(resource, params);
  },
  update: (resource, params) => {
    if (resource === 'posts') {
      const formData = createPostFormData(params);
      formData.append('id', params.id);
      return fetchUtils
        .fetchJson(`${endpoint}/${resource}`, {
          method: 'PUT',
          body: formData,
        })
        .then(({ json }) => ({ data: json }));
    }

    return baseDataProvider.update(resource, params);
  },
};

export default dataProvider;
