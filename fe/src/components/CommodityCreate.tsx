import {
  ReferenceInput,
  Create,
  SimpleForm,
  TextInput,
  required,
  SelectInput,
} from 'react-admin';
import React from 'react';
import { COMMODITY_TYPES_ELEMENTS } from '../api/types/commodity-types';
import { COMMODITY_STATUS_ELEMENTS } from '../api/types/commodity-statuses';

function CommodityCreate() {
  return (
    <Create>
      <SimpleForm>
        <TextInput source="name" validate={[required()]} label="Name" fullWidth name="name" />
        <SelectInput
          source="type"
          validate={[required()]}
          label="type"
          name="type"
          choices={COMMODITY_TYPES_ELEMENTS}
        />
        <ReferenceInput source="area_id" validate={[required()]} reference="areas" name="area" />
        <SelectInput
          source="status"
          validate={[required()]}
          label="status"
          name="status"
          choices={COMMODITY_STATUS_ELEMENTS}
        />
      </SimpleForm>
    </Create>
  );
}

export default CommodityCreate;
