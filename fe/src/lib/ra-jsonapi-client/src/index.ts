// src/dataProvider.ts

import merge from 'deepmerge';
import axios, { AxiosInstance } from 'axios';
import { stringify } from 'qs';
import { DataProvider } from 'ra-core';
import defaultSettings from './default-settings';

interface DataProviderOptions {
  apiUrl: string;
  userSettings?: typeof defaultSettings;
}

interface LooseObject {
  [key: string]: any
}

const createDataProvider = (options: DataProviderOptions): DataProvider => {
  const settings = merge(defaultSettings, options.userSettings || {});

  const httpClient: AxiosInstance = axios.create({
    baseURL: options.apiUrl,
    // Add more Axios configuration here (headers, etc.) as needed
  });

  const getTotal = (responseData: any): number => {
    let total;

    if (responseData.meta && settings.total) {
      total = responseData.meta[settings.total];
    }

    return total || responseData.data.length;
  };

  const getListData = (responseData: any): any => {
    return responseData.data.map((value_2: LooseObject) => ({
      id: value_2.id,
      ...value_2.attributes,
    }));
  };

  const getList = async (resource: string, params: any): Promise<any> => {
    const { page, perPage } = params.pagination;
    const { field, order } = params.sort;
    const query = {
      sort: [`${order === 'ASC' ? '' : '-'}${field}`],
      page,
      'page[limit]': perPage,
      filter: params.filter,
    };
    const url = `/${resource}?${stringify(query)}`;
    const response = await httpClient.get(url);

    return {
      data: getListData(response.data),
      total: getTotal(response.data),
    };
  };

  const getOne = async (resource: string, params: any): Promise<any> => {
    const response = await httpClient.get(`/${resource}/${params.id}`);

    const { id, attributes: attributes1 } = response.data.data;

    return {
      data: {
        _meta: response.data.data.meta,
        id,
        ...attributes1,
      },
    };
  };

  const getMany = async (resource: string, params: any): Promise<any> => {
    const query = {
      filter: { id: params.ids },
    };
    const url = `/${resource}?${stringify(query)}`;
    const response = await httpClient.get(url);
    return {
      data: getListData(response.data),
      total: getTotal(response.data),
    };
  };

  const getManyReference = async (resource: string, params: any): Promise<any> => {
    const { page, perPage } = params.pagination;
    const { field, order } = params.sort;
    const query = {
      sort: [`${order === 'ASC' ? '' : '-'}${field}`],
      page,
      'page[limit]': perPage,
      filter: { ...params.filter, [params.target]: params.id },
    };
    const url = `/${resource}?${stringify(query)}`;
    const response = await httpClient.get(url);
    return {
      data: getListData(response.data),
      total: getTotal(response.data),
    };
  };

  const create = async (resource: string, params: any): Promise<any> => {
    const response = await httpClient.post(
      `/${resource}`,
      {
        data: {
          type: resource,
          attributes: params.data,
        },
      },
    );
    return {
      data: { ...params.data, id: response.data.data.id },
    };
  };

  const update = async (resource: string, params: any): Promise<any> => {
    const attributes = params.data;
    delete attributes.id;
    const data = {
      data: {
        id: params.id,
        type: resource,
        attributes,
      },
    };

    const response = await httpClient.put(`/${resource}/${params.id}`, data);
    return {
      data: { ...params.data, id: response.data.data.id },
    };
  };

  const updateMany = async (resource: string, params: any): Promise<any> => {
    const responses = await Promise.all(
      params.ids.map((id: any) => httpClient.put(`/${resource}/${id}`, params.data)),
    );
    return { data: responses.map((res) => res.data.data.id) };
  };

  const del = async (resource: string, params: any): Promise<any> => {
    await httpClient.delete(`/${resource}/${params.id}`);
    return { data: params.previousData };
  };

  const deleteMany = async (resource: string, params: any): Promise<any> => {
    const responses = await Promise.all(
      params.ids.map((id: any) => httpClient.delete(`/${resource}/${id}`)),
    );
    return { data: responses.map((res) => res.data.data.id) };
  };

  return {
    getList,
    getOne,
    getMany,
    getManyReference,
    create,
    update,
    updateMany,
    delete: del,
    deleteMany,
  };
};

export default createDataProvider;
