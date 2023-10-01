import {
  ReferenceInput,
  Create,
  SimpleForm,
  TextInput,
  required,
} from 'react-admin';
import React from 'react';

function CommodityCreate() {
  return (
    <Create>
      <SimpleForm>
        <TextInput source="name" validate={[required()]} label="Name" fullWidth name="name" />
        <TextInput source="type" validate={[required()]} label="type" fullWidth name="type" />
        <ReferenceInput source="area_id" validate={[required()]} reference="areas" name="area" />
        <TextInput source="status" validate={[required()]} label="status" fullWidth name="status" />
      </SimpleForm>
    </Create>
  );
}

export default CommodityCreate;
