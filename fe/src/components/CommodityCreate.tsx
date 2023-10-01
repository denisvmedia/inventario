import {
  ReferenceInput,
  Create,
  SimpleForm,
  TextInput,
  required,
  SelectInput,
  DateInput,
  NumberInput,
  BooleanInput,
} from 'react-admin';
import React from 'react';
import { COMMODITY_TYPES_ELEMENTS } from '../api/types/commodity-types';
import {COMMODITY_STATUS_ELEMENTS, COMMODITY_STATUS_IN_USE} from '../api/types/commodity-statuses';

function CommodityCreate() {
  return (
    <Create>
      <SimpleForm>
        <TextInput
          source="name"
          validate={[required()]}
          label="Name"
          fullWidth
          name="name"
        />
        <TextInput
          source="short_name"
          validate={[required()]}
          label="Short Name"
          fullWidth
          name="short_name"
        />
        <SelectInput
          source="type"
          validate={[required()]}
          label="type"
          name="type"
          choices={COMMODITY_TYPES_ELEMENTS}
        />
        <ReferenceInput
          source="area_id"
          validate={[required()]}
          reference="areas"
          name="area"
        />
        <NumberInput
          source="count"
          validate={[required()]}
          label="Count"
          name="count"
          min={1}
          defaultValue={1}
        />
        <SelectInput
          source="status"
          validate={[required()]}
          label="status"
          name="status"
          choices={COMMODITY_STATUS_ELEMENTS}
          defaultValue={COMMODITY_STATUS_IN_USE}
        />
        <DateInput
          source="purchase_date"
          validate={[required()]}
          label="Purchase Date"
          name="purchase_date"
          defaultValue={new Date().toISOString().slice(0, 10)}
        />
        <TextInput source="urls" multiline label="URLs" name="urls" fullwidth />
        <BooleanInput
          source="draft"
          label="Draft"
          name="draft"
          defaultValue={false}
        />
      </SimpleForm>
    </Create>
  );
}

export default CommodityCreate;
