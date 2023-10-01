import React, { useEffect, useState } from 'react';
import { useRecordContext, useDataProvider } from 'react-admin';
import Chip from '@mui/material/Chip';
import Paper from '@mui/material/Paper';
import InputLabel from '@mui/material/InputLabel';
import { styled } from '@mui/material/styles';

const ListItem = styled('li')(({ theme }) => ({
  margin: theme.spacing(0.5),
}));

function ReadOnlyReferenceArrayInput({ source, reference, label }: any) {
  const record = useRecordContext();
  const selectedIds = record[source] || [];
  const [selectedRecords, setSelectedRecords] = useState([]);
  const dataProvider = useDataProvider();

  if (selectedIds.length === 0) return null;

  useEffect(() => {
    if (selectedIds.length > 0) {
      // Fetch the related records based on the selectedIds
      dataProvider
        .getMany(reference, {
          ids: selectedIds,
        })
        .then(({ data }: any) => setSelectedRecords(data));
    }
  }, [dataProvider, reference, selectedIds]);

  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        flexWrap: 'wrap',
        listStyle: 'none',
        margin: 0,
        position: 'relative',
      }}
    >
      <InputLabel
        size="small"
        shrink
        sx={{
          position: 'absolute',
          transfromOrigin: 'top left',
          overflow: 'hidden',
          top: 0,
          left: 0,
          transform: 'translate(14px, -9px) scale(0.75)',
          padding: '0 2px',
          background: 'white',
        }}
      >
        <span>{label}</span>
      </InputLabel>
      <Paper
        variant="outlined"
        sx={{
          display: 'flex',
          justifyContent: 'center',
          flexWrap: 'wrap',
          listStyle: 'none',
          p: 0.5,
          m: 0,
        }}
        component="ul"
      >
        {selectedRecords.map((record: any) => (
          <ListItem key={record.id}>
            <Chip
              key={record.id}
              label={record.name}
              variant="filled"
              size="small"
              sx={{ margin: '3px' }}
            />
          </ListItem>
        ))}
      </Paper>
    </div>
  );
}

export default ReadOnlyReferenceArrayInput;
